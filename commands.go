// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"
	"strings"

	"github.com/gobwas/glob"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	// Message to be sent when fallacy does not have sufficient permissions.
	permsMessage = "Fallacy does not have sufficient permission to perform that action!"

	// Usage message to be sent when asking for help.
	usage = "Hey, check out the usage guide at https://github.com/qua3k/fallacy/blob/main/USAGE.md"
)

type Callback struct {
	Function func(command []string, event event.Event)
	Min      int
}

// Register registers a command with a keyword.
func Register(keyword string, callback Callback) {
	lock.Lock()
	defer lock.Unlock()

	keyword = strings.ToLower(keyword)
	if _, ok := handles[keyword]; !ok {
		handles[keyword] = []Callback{}
	}
	handles[keyword] = append(handles[keyword], callback)
}

// notifyListeners notifies listeners of incoming events.
func notifyListeners(command []string, ev event.Event) {
	if len(command) < 2 || strings.EqualFold(command[1], "help") {
		if _, err := sendReply(ev, usage); err != nil {
			log.Println("could not send reply into room, failed with:", err)
		}
		return
	}

	if !isAdmin(ev.RoomID, ev.Sender) {
		if _, err := sendReply(ev, "shut up ur not admin"); err != nil {
			log.Println("could not send reply into room, failed with:", err)
		}
		return
	}

	if c, ok := handles[strings.ToLower(command[1])]; ok {
		for i := range c {
			args := command[2:]
			if len(args) < c[i].Min {
				sendNotice(ev.RoomID, "not enough arguments!")
				continue
			}
			go c[i].Function(args, ev)
		}
		return
	}
	sendNotice(ev.RoomID, command[1]+" is not a valid command!")
}

// sendNotice is a wrapper around Client.SendNotice that logs when sending a
// notice fails.
func sendNotice(roomID id.RoomID, text ...string) (resp *mautrix.RespSendEvent) {
	<-limit
	resp, err := Client.SendNotice(roomID, strings.Join(text, " "))
	if err != nil {
		log.Println("could not send notice into room", roomID, "failed with error:", err)
	}
	return
}

// sendReply sends a message as a reply to another message.
func sendReply(ev event.Event, s string) (*mautrix.RespSendEvent, error) {
	<-limit
	return Client.SendMessageEvent(ev.RoomID, event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    s,
		RelatesTo: &event.RelatesTo{
			Type:    event.RelReply,
			EventID: ev.ID,
		},
	})
}

func minAdmin(pl *event.PowerLevelsEventContent) (min int) {
	min = pl.Ban()

	k, r := pl.Kick(), pl.Redact()
	if k < min {
		min = k
	}
	if r < min {
		min = r
	}
	return min
}

// isAdmin returns whether the user is a room admin by checking ban/kick/redact
// power levels.
func isAdmin(roomID id.RoomID, userID id.UserID) bool {
	pl, err := powerLevels(roomID)
	if err != nil {
		log.Println("fetching power levels event failed!")
		return false
	}

	return pl.GetUserLevel(userID) >= minAdmin(pl)
}

// hasPerms checks whether the fallacy bot has perms.
func hasPerms(roomID id.RoomID, event event.Type) bool {
	pl, err := powerLevels(roomID)
	if err != nil {
		log.Println("fetching power levels event failed with error", err)
		return false
	}

	if pl.GetEventLevel(event) > pl.GetUserLevel(Client.UserID) {
		return false
	}
	return true
}

// BanServer bans a server by adding it to the room ACL.
func BanServer(roomID id.RoomID, homeserver string) (err error) {
	if !hasPerms(roomID, event.StateServerACL) {
		sendNotice(roomID, permsMessage)
		return
	}

	glb, err := glob.Compile(homeserver)
	if err != nil {
		sendNotice(roomID, "not a valid glob pattern!")
		return
	}

	_, hs, _ := Client.UserID.Parse()
	if !glb.Match(hs) {
		sendNotice(roomID, "Refusing to ban own homeserver...")
		return
	}

	acls, err := acls(roomID)
	if err != nil {
		return
	}

	for _, s := range acls.Allow {
		if s == homeserver {
			return
		}
	}

	for _, server := range acls.Deny {
		if server == homeserver {
			return
		}
	}

	acls.Deny = append(acls.Deny, homeserver)
	_, err = Client.SendStateEvent(roomID, event.StateServerACL, "", &acls)
	return
}

// MuteUser mutes a target user in a specified room by utilizing power levels.
func MuteUser(body []string, ev event.Event) {
	pl, err := powerLevels(ev.RoomID)
	if err != nil {
		log.Println(err)
		return
	}

	targetID := id.UserID(body[0])

	level := pl.GetEventLevel(event.EventMessage)
	if pl.GetUserLevel(targetID) < level {
		sendNotice(ev.RoomID, "cannot mute a user that is already muted")
		return
	}
	pl.SetUserLevel(targetID, level)
	if _, err := Client.SendStateEvent(ev.RoomID, event.StatePowerLevels, "", &pl); err != nil {
		sendNotice(ev.RoomID, "could not mute user! failed with: "+err.Error())
		return
	}
	msg := strings.Join([]string{body[0], "was muted by", ev.Sender.String(), "in", ev.RoomID.String()}, " ")
	sendNotice(ev.RoomID, msg)
	return
}

// UnmuteUser unmutes a target user in a specified room by utilizing power levels.
func UnmuteUser(body []string, ev event.Event) {
	pl, err := powerLevels(ev.RoomID)
	if err != nil {
		log.Println(err)
		return
	}

	targetID := id.UserID(body[0])

	level := pl.GetEventLevel(event.EventMessage)
	if pl.GetUserLevel(targetID) >= level {
		sendNotice(ev.RoomID, "cannot unmute a user that is not muted")
		return
	}
	pl.SetUserLevel(targetID, level)
	if _, err := Client.SendStateEvent(ev.RoomID, event.StatePowerLevels, "", &pl); err != nil {
		sendNotice(ev.RoomID, "could not unmute user! failed with: "+err.Error())
		return
	}
	msg := strings.Join([]string{body[0], "was unmuted by", ev.Sender.String(), "in", ev.RoomID.String()}, " ")
	sendNotice(ev.RoomID, msg)
	return
}

// PinMessage pins the replied-to event.
func PinMessage(body []string, ev event.Event) {
	if !hasPerms(ev.RoomID, event.StatePinnedEvents) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	relatesTo := ev.Content.AsMessage().RelatesTo
	if relatesTo == nil {
		sendNotice(ev.RoomID, "Reply to the message you want to pin!")
		return
	}

	p := event.PinnedEventsEventContent{}
	// Avoid handling this error. The pinned event may not exist.
	Client.StateEvent(ev.RoomID, event.StatePinnedEvents, "", &p)

	p.Pinned = append(p.Pinned, relatesTo.EventID)
	Client.SendStateEvent(ev.RoomID, event.StatePinnedEvents, "", &p)
}

// SayMessage sends a message into the chat.
func SayMessage(body []string, ev event.Event) {
	msg := strings.Join(body, " ")
	sendNotice(ev.RoomID, msg)
}
