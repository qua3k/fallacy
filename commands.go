// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// The max amount of messages to fetch at once—the server will only give about
// ~1000 events.
const fetchLimit = 1000
const permsMessage = "Fallacy does not have sufficient permission to perform that action!"

type listener func(command []string, event event.Event)
type Callback struct {
	Function listener
	Min      int
}

// Register registers a command with a keyword.
func (f *Fallacy) Register(keyword string, callback Callback) {
	keyword = strings.ToLower(keyword)

	if _, ok := f.Handlers[keyword]; !ok {
		f.Handlers[keyword] = []Callback{}
	}
	f.Handlers[keyword] = append(f.Handlers[keyword], callback)
}

// notifyListeners notifies listeners of incoming events.
func (f *Fallacy) notifyListeners(command []string, ev event.Event) {
	if len(command) < 2 || strings.EqualFold(command[1], "help") {
		if _, err := f.sendReply(ev, usage); err != nil {
			log.Println("could not send reply into room, failed with:", err)
		}
		f.printHelp(ev.RoomID)
		return
	}

	if !f.isAdmin(ev.RoomID, ev.Sender) {
		if _, err := f.sendReply(ev, "shut up ur not admin"); err != nil {
			log.Println("could not send reply into room, failed with:", err)
		}
		return
	}

	if c, ok := f.Handlers[strings.ToLower(command[1])]; ok {
		for i := range c {
			args := command[2:]
			if len(args) < c[i].Min {
				continue
			}
			go c[i].Function(args, ev)
		}
		return
	}
	f.attemptSendNotice(ev.RoomID, command[1]+" is not a valid command!")
}

func isAdmin(pl *event.PowerLevelsEventContent, roomID id.RoomID, userID id.UserID) bool {
	bp, kp, rp := pl.Ban(), pl.Kick(), pl.Redact()

	minInt := func(i ...int) (m int) {
		m = i[0]
		for _, v := range i {
			if v < m {
				m = v
			}
		}
		return
	}

	return pl.GetUserLevel(userID) >= minInt(bp, kp, rp)
}

// isAdmin returns whether the user is a room admin by checking ban/kick/redact
// power levels.
func (f *Fallacy) isAdmin(roomID id.RoomID, userID id.UserID) bool {
	pl, err := f.powerLevels(roomID)
	if err != nil {
		log.Println("fetching power levels event failed!")
		return false
	}

	return isAdmin(&pl, roomID, userID)
}

// Checks whether the fallacy bot has perms.
func (f *Fallacy) hasPerms(roomID id.RoomID, event event.Type) bool {
	pl, err := f.powerLevels(roomID)
	if err != nil {
		log.Println("fetching power levels event failed!")
		return false
	}

	if pl.GetEventLevel(event) > pl.GetUserLevel(f.Client.UserID) {
		return false
	}
	return true
}

// BanServer bans a server by adding it to the room ACL.
func (f *Fallacy) BanServer(roomID id.RoomID, homeserverID string) (err error) {
	if !f.hasPerms(roomID, event.StateServerACL) {
		f.attemptSendNotice(roomID, permsMessage)
		return
	}

	acls, err := f.acls(roomID)
	if err != nil {
		return
	}

	for _, server := range acls.Allow {
		if server == homeserverID {
			return
		}
	}

	for _, server := range acls.Deny {
		if server == homeserverID {
			return
		}
	}

	acls.Deny = append(acls.Deny, homeserverID)
	_, err = f.Client.SendStateEvent(roomID, event.StateServerACL, "", &acls)
	return
}

// MuteUser mutes a target user in a specified room by utilizing power levels.
func (f *Fallacy) MuteUser(roomID id.RoomID, targetID id.UserID) (err error) {
	pl, err := f.powerLevels(roomID)
	if err != nil {
		return
	}

	level := pl.GetEventLevel(event.EventMessage) - 1
	if pl.GetUserLevel(targetID) <= level {
		const e = "cannot mute a user that is not muted"
		f.attemptSendNotice(roomID, e)
		return errors.New(e)
	}

	pl.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", &pl)
	return
}

// MuteUsers mutes multiple users of a slice.
// This could probably be optimized.
func (f *Fallacy) MuteUsers(users []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.StatePowerLevels) {
		f.attemptSendNotice(ev.RoomID, permsMessage)
		return
	}

	for _, u := range users {
		if err := f.MuteUser(ev.RoomID, id.UserID(u)); err != nil {
			log.Println("muting user", u, "failed with", err)
			continue
		}

		r, err := f.roomName(ev.RoomID)
		if err != nil {
			log.Println("could not get room name, failed with", err)
		}

		n := ev.RoomID.String()
		if r.Name != "" && r.Name != " " {
			n = r.Name
		}
		msg := strings.Join([]string{u, "was muted by", ev.Sender.String(), "in", n}, " ")
		f.attemptSendNotice(ev.RoomID, msg)
	}
}

// UnmuteUser unmutes a target user in a specified room by utilizing power levels.
func (f *Fallacy) UnmuteUser(roomID id.RoomID, targetID id.UserID) (err error) {
	pl, err := f.powerLevels(roomID)
	if err != nil {
		return
	}

	level := pl.GetEventLevel(event.EventMessage)
	if pl.GetUserLevel(targetID) >= level {
		const e = "cannot unmute a user that is not muted"
		f.attemptSendNotice(roomID, e)
		return errors.New(e)
	}

	pl.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", &pl)
	return
}

// UnmuteUsers mutes multiple users of a slice.
// This could probably be optimized.
func (f *Fallacy) UnmuteUsers(users []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.StatePowerLevels) {
		f.attemptSendNotice(ev.RoomID, permsMessage)
		return
	}

	for _, u := range users {
		if err := f.UnmuteUser(ev.RoomID, id.UserID(u)); err != nil {
			log.Println("unmuting user", u, "failed with", err)
			continue
		}

		r, err := f.roomName(ev.RoomID)
		if err != nil {
			log.Println("could not get room name, failed with", err)
		}

		n := ev.RoomID.String()
		if r.Name != "" && r.Name != " " {
			n = r.Name
		}
		msg := strings.Join([]string{u, "was muted by", ev.Sender.String(), "in", n}, " ")
		f.attemptSendNotice(ev.RoomID, msg)
	}
}

// PinMessage pins the replied-to event.
func (f *Fallacy) PinMessage(_ []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.StatePinnedEvents) {
		f.attemptSendNotice(ev.RoomID, permsMessage)
		return
	}

	relatesTo := ev.Content.AsMessage().RelatesTo
	if relatesTo == nil {
		f.attemptSendNotice(ev.RoomID, "Reply to the message you want to pin!")
		return
	}

	p := event.PinnedEventsEventContent{}
	// Avoid handling this error. The pinned event may not exist.
	f.Client.StateEvent(ev.RoomID, event.StatePinnedEvents, "", &p)

	p.Pinned = append(p.Pinned, relatesTo.EventID)
	f.Client.SendStateEvent(ev.RoomID, event.StatePinnedEvents, "", &p)
}

// SayMessage sends a message into the chat.
func (f *Fallacy) SayMessage(message []string, ev event.Event) {
	msg := strings.Join(message, " ")
	f.attemptSendNotice(ev.RoomID, msg)
}
