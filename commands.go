// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"strings"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type commandListener func(command []string, event event.Event)

type CallbackStruct struct {
	Function commandListener
	MinArgs  int
}

// Register adds a function to the map.
func (f *Fallacy) Register(keyword string, callback CallbackStruct) {
	_, ok := f.Handlers[keyword]
	if !ok {
		f.Handlers[keyword] = []CallbackStruct{}
	}
	f.Handlers[keyword] = append(f.Handlers[keyword], callback)
}

// notifyListeners notifies listeners of incoming events.
func (f *Fallacy) notifyListeners(command []string, event event.Event) {
	roomID := event.RoomID

	if len := len(command); len < 2 {
		f.printHelp(roomID)
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
	if strings.EqualFold(action, "help") {
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

func parseMessage(ev event.Event) (id.RoomID, id.UserID) {
	return ev.RoomID, ev.Sender
}

// BanServer bans a server by adding it to the room ACL.
func (f *Fallacy) BanServer(roomID id.RoomID, homeserverID string) (err error) {
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
	if roomID, userID := parseMessage(ev); !f.isAdmin(roomID, userID) {
		return
	}
	for _, u := range users {
		roomID, userID := ev.RoomID, id.UserID(u)
		if err := f.MuteUser(roomID, userID); err != nil {
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
	if roomID, userID := parseMessage(ev); !f.isAdmin(roomID, userID) {
		return
	}
	for _, u := range users {
		roomID, userID := ev.RoomID, id.UserID(u)
		if err := f.UnmuteUser(roomID, userID); err != nil {
			log.Println("muting user", u, "failed with", err)
		}
	}
}

// RedactMessage only redacts message events, skipping redaction events, already
// redacted events, and state events.
func (f *Fallacy) RedactMessage(ev event.Event) (err error) {
	switch {
	case ev.StateKey == nil,
		ev.Type != event.EventRedaction,
		ev.Unsigned.RedactedBecause == nil:
		_, err = f.Client.RedactEvent(ev.RoomID, ev.ID, mautrix.ReqRedact{})
	}
	return
}

func (f *Fallacy) PurgeUser(users []string, ev event.Event) {
	roomID, senderID := parseMessage(ev)
	if !f.isAdmin(roomID, senderID) {
		return
	}

	var wg sync.WaitGroup
	redactWait := func(ev event.Event) {
		wg.Add(1)
		defer wg.Done()
		if err := f.RedactMessage(ev); err != nil {
			log.Println("redacting message failed with error:", err)
		}
	}

	const maxFetchLimit = 2147483647
	filter := setupPurgeFilter()

	var (
		msg *mautrix.RespMessages
		err error
	)

	purge := func(token string) {
		msg, err = f.Client.Messages(roomID, "", "", 'b', &filter, maxFetchLimit)
		if err != nil {
			log.Println(err)
			return
		}

		for _, e := range msg.Chunk {
			if e == nil {
				continue
			}
			for _, u := range users {
				if e.Sender != id.UserID(u) {
					continue
				}
				redactWait(*e)
			}
		}
	}

	purge("")
	for msg.End != "" {
		purge(msg.End)
	}

	wg.Wait()
}

// PurgeMessages redacts all message events newer than the specified event ID.
// It's loosely inspired by Telegram's SophieBot mechanics.
func (f *Fallacy) PurgeMessages(_ []string, ev event.Event) {
	roomID, senderID := parseMessage(ev)
	if !f.isAdmin(roomID, senderID) {
		return
	}

	var wg sync.WaitGroup
	const maxFetchLimit = 2147483647 // the maximum number of messages to fetch.

	redactWait := func(ev event.Event) {
		wg.Add(1)
		defer wg.Done()
		if err := f.RedactMessage(ev); err != nil {
			log.Println("redacting message failed with error:", err)
		}
	}

	relatesTo := ev.Content.AsMessage().RelatesTo
	if relatesTo == nil {
		return
	}

	purgeMessages := func(s []*event.Event) {
		for _, e := range s {
			if e == nil {
				continue
			}
			go redactWait(*e) // check for races
		}
	}

	filter := setupPurgeFilter()
	con, err := f.Client.Context(roomID, relatesTo.EventID, &filter, 1)
	if err != nil {
		log.Println(err)
		return
	}

	if con.Event != nil {
		go redactWait(*con.Event)
	}
	purgeMessages(con.EventsAfter)

	msg, err := f.Client.Messages(roomID, con.End, "", 'f', &filter, maxFetchLimit)
	if err != nil {
		log.Println(err)
		return
	}
	purgeMessages(msg.Chunk)
	wg.Wait()
}

func (f *Fallacy) SayMessage(message []string, ev event.Event) {
	roomID, userID := parseMessage(ev)
	if !f.isAdmin(roomID, userID) {
		f.attemptSendNotice(roomID, "shut up ur not admin")
		return
	}
	msg := strings.Join(message, " ")
	f.attemptSendNotice(roomID, msg)
}
