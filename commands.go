// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Commands struct{}

// powerLevels returns a power levels struct.
func (f *Fallacy) powerLevels(roomID id.RoomID) (resp event.PowerLevelsEventContent, err error) {
	err = f.Client.StateEvent(roomID, event.StatePowerLevels, "", resp)
	return
}

// isAdmin returns whether the user is a room admin by checking ban/kick/redact
// power levels.
func (f *Fallacy) isAdmin(roomID id.RoomID, userID id.UserID) bool {
	pl, err := f.powerLevels(roomID)
	if err != nil {
		log.Println("fetching power levels event failed!")
		return false
	}

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

// MuteUser mutes a target user in a specified room by utilizing power levels.
func (f *Fallacy) MuteUser(roomID id.RoomID, targetID id.UserID) (err error) {
	pl, err := f.powerLevels(roomID)
	if err != nil {
		return
	}
	level := pl.GetEventLevel(event.EventMessage)
	if pl.GetUserLevel(targetID) <= level-1 {
		return errors.New("cannot mute a user that is already muted")
	}
	pl.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", &pl)
	return
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

// RedactMessage only redacts message events, skipping redaction events, already
// redacted events, and state events. Rather than checking for the presence of
// keys in the content object, we check for the presence of the redacted_because
// object to ensure future compatibility with e2ee.
func (f *Fallacy) RedactMessage(ev *event.Event, wait *sync.WaitGroup) {
	defer wait.Done()
	if ev.StateKey == nil ||
		ev.Type != event.EventRedaction ||
		ev.Unsigned.RedactedBecause == nil {
		if _, err := f.Client.RedactEvent(ev.RoomID, ev.ID, mautrix.ReqRedact{}); err != nil {
			log.Println("redacting message failed with error:", err)
		}
	}
}

// PurgeMessages redacts all message events newer than the specified event ID.
// It's loosely inspired by Telegram's SophieBot mechanics.
//
// TODO: check for races
func (f *Fallacy) PurgeMessages(roomID id.RoomID, eventID id.EventID) error {
	var wg sync.WaitGroup

	purgeMessages := func(s []*event.Event) {
		for _, e := range s {
			if e == nil {
				continue
			}
			wg.Add(1)
			go f.RedactMessage(e, &wg)
		}
	}

	filter := setupPurgeFilter()
	con, err := f.Client.Context(roomID, eventID, &filter, 1)
	if err != nil {
		return err
	}

	wg.Add(1)
	go f.RedactMessage(con.Event, &wg)
	purgeMessages(con.EventsAfter)

	msg, err := f.Client.Messages(roomID, con.End, "", 'f', &filter, 2147483647) // will the server give us all these events?
	if err != nil {
		return err
	}
	purgeMessages(msg.Chunk)

	wg.Wait()
	return nil
}
