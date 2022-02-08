// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// The max amount of messages to fetch at once—the server will only give about
// ~1000 events.
const maxFetchLimit = 1000
const adminMessage = "shut up ur not admin"
const noPermsMessage = "Fallacy does not have sufficient permission to perform that action!"

type commandListener func(command []string, event event.Event)

type Callback struct {
	Function commandListener
	MinArgs  int
}

// Register adds a function to the map.
func (f *Fallacy) Register(keyword string, callback Callback) {
	_, ok := f.Handlers[keyword]
	if !ok {
		f.Handlers[keyword] = []Callback{}
	}
	f.Handlers[keyword] = append(f.Handlers[keyword], callback)
}

// notifyListeners notifies listeners of incoming events.
func (f *Fallacy) notifyListeners(command []string, event event.Event) {
	roomID := event.RoomID

	if l := len(command); l < 2 {
		f.printHelp(roomID)
		return
	}

	if !f.isAdmin(roomID, event.Sender) {
		f.attemptSendNotice(roomID, adminMessage)
		return
	}

	action := command[1]
	for keyword, listen := range f.Handlers {
		if !strings.EqualFold(action, keyword) {
			continue
		}

		for _, s := range listen {
			input := command[2:]
			if len(input) < s.MinArgs {
				continue
			}
			s.Function(input, event)
		}
		return
	}
	if !strings.EqualFold(action, "help") {
		f.attemptSendNotice(roomID, action+" is not a valid command!")
	}
	f.printHelp(roomID)
}

// powerLevels returns a power levels struct from the specified roomID.
func (f *Fallacy) powerLevels(roomID id.RoomID) (resp event.PowerLevelsEventContent, err error) {
	err = f.Client.StateEvent(roomID, event.StatePowerLevels, "", &resp)
	return
}

// acls returns an ACL struct.
func (f *Fallacy) acls(roomID id.RoomID) (resp event.ServerACLEventContent, err error) {
	err = f.Client.StateEvent(roomID, event.StateServerACL, "", &resp)
	return
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
		f.attemptSendNotice(roomID, noPermsMessage)
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
		return errors.New("cannot mute a user that is already muted")
	}

	pl.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", &pl)
	return
}

// MuteUsers mutes multiple users of a slice.
// This could probably be optimized.
func (f *Fallacy) MuteUsers(users []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.StateServerACL) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	for _, u := range users {
		if err := f.MuteUser(ev.RoomID, id.UserID(u)); err != nil {
			log.Println("muting user", u, "failed with", err)
		}
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
		return errors.New("cannot unmute a user that is not muted")
	}

	pl.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", &pl)
	return
}

// UnmuteUsers mutes multiple users of a slice.
// This could probably be optimized.
func (f *Fallacy) UnmuteUsers(users []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.StateServerACL) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	for _, u := range users {
		if err := f.UnmuteUser(ev.RoomID, id.UserID(u)); err != nil {
			log.Println("unmuting user", u, "failed with", err)
		}
	}
}

// PinMessage pins the replied-to event.
func (f *Fallacy) PinMessage(_ []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.StatePinnedEvents) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
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

// RedactMessage only redacts message events, skipping redaction events, already
// redacted events, and state events.
func (f *Fallacy) RedactMessage(ev event.Event) (err error) {
	if ev.StateKey != nil {
		return
	}

	if ev.Type != event.EventRedaction && ev.Unsigned.RedactedBecause == nil {
		_, err = f.Client.RedactEvent(ev.RoomID, ev.ID, mautrix.ReqRedact{})
	}
	return
}

func (f *Fallacy) purgeEvents(evs []*event.Event) {
	for _, e := range evs {
		if e == nil {
			continue
		}
		go f.RedactMessage(*e)
	}
}

func (f *Fallacy) redactUsers(users []string, ev event.Event) {
	for _, u := range users {
		if id.UserID(u) != ev.Sender {
			continue
		}
		go f.RedactMessage(ev)
	}
}

// PurgeUsers redacts all messages sent by the specified users.
func (f *Fallacy) PurgeUsers(users []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.EventRedaction) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	filter := purgeUserFilter(users)
	msg, err := f.Client.Messages(ev.RoomID, "", "", 'b', &filter, maxFetchLimit)
	if err != nil {
		log.Println(err)
		return
	}

	var prevEnd string
	for msg.End != prevEnd {
		prevEnd = msg.End
		for _, e := range msg.Chunk {
			if e == nil {
				continue
			}
			go f.redactUsers(users, *e)
		}
		msg, err = f.Client.Messages(ev.RoomID, msg.End, "", 'b', &filter, maxFetchLimit)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

// PurgeMessages redacts all message events newer than the specified event ID.
// It's loosely inspired by Telegram's SophieBot mechanics.
func (f *Fallacy) PurgeMessages(_ []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.EventRedaction) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	relatesTo := ev.Content.AsMessage().RelatesTo
	if relatesTo == nil {
		f.attemptSendNotice(ev.RoomID, "Reply to the message you want to purge!")
		return
	}

	c, err := f.Client.Context(ev.RoomID, relatesTo.EventID, &purgeFilter, 1)
	if err != nil {
		log.Println(err)
		return
	}

	if c.Event == nil {
		log.Println("event is nil (should this happen)?")
		return
	}
	go f.RedactMessage(*c.Event)
	go f.purgeEvents(c.EventsAfter)

	msg, err := f.Client.Messages(ev.RoomID, c.End, "", 'f', &purgeFilter, maxFetchLimit)
	if err != nil {
		log.Println(err)
		return
	}
	go f.purgeEvents(msg.Chunk)
}

// SayMessage sends a message into the chat.
func (f *Fallacy) SayMessage(message []string, ev event.Event) {
	msg := strings.Join(message, " ")
	f.attemptSendNotice(ev.RoomID, msg)
}
