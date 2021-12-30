// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// MinimumFetch is the minimum number of messages to fetch during a request to `/messages`.
const MinimumFetch int = 20

/* type Command struct{
	listeners map[string]listener
}

type listener func(body string)

func (c *Command) Register(keyword string) {

} */

// powerLevels returns a power levels struct.
func (f *Fallacy) powerLevels(roomID id.RoomID) (resp *event.PowerLevelsEventContent, err error) {
	err = f.Client.StateEvent(roomID, event.StatePowerLevels, "", &resp)
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
func (f *Fallacy) MuteUser(roomID id.RoomID, senderID, targetID id.UserID) (err error) {
	pwr, err := f.powerLevels(roomID)
	if err != nil {
		return
	}
	level := pwr.GetEventLevel(event.EventMessage)
	if pwr.GetUserLevel(targetID) <= level-1 {
		return errors.New("cannot mute a user that is already muted")
	}
	pwr.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", pwr)
	return
}

// UnmuteUser unmutes a target user in a specified room by utilizing power levels.
func (f *Fallacy) UnmuteUser(roomID id.RoomID, senderID, targetID id.UserID) (err error) {
	pwr, err := f.powerLevels(roomID)
	if err != nil {
		return
	}
	level := pwr.GetEventLevel(event.EventMessage)
	if pwr.GetUserLevel(targetID) >= level {
		return errors.New("cannot unmute a user that is not muted")
	}
	pwr.SetUserLevel(targetID, level)
	_, err = f.Client.SendStateEvent(roomID, event.StatePowerLevels, "", pwr)
	return
}

// PurgeMessages redacts a number of room events in a room, optionally ending at
// a specific pagination token obtained from a previous request to the endpoint.
//
// Alternative solutions were evaluated such as deleting all messages starting
// from a message that was replied to the newest message à la Telegram's
// SophieBot but they were determined to be infeasible as calculating the
// pagination tokens were specific to Synapse and not exposed through the
// client-server API.
func (f *Fallacy) PurgeMessages(roomID id.RoomID, end string, limit int) error {
	var (
		fetchNum    int    = limit
		purgedCount uint32 = uint32(limit)
		wg          sync.WaitGroup
	)

	if limit <= 0 {
		return errors.New("nothing to purge...")
	}

	if fetchNum < MinimumFetch {
		fetchNum = MinimumFetch // fetchNum must never be less than MinimumFetch
	}

	resp, err := f.Client.Messages(roomID, end, "", 'b', fetchNum)
	if err != nil {
		return err
	}

	chunkSize := len(resp.Chunk)
	if limit < chunkSize {
		chunkSize = limit // we may fetch more events than we actually need
	}

	for _, e := range resp.Chunk[0:chunkSize] {
		wg.Add(1)
		go func(e event.Event) { // check for races
			defer wg.Done()
			if e.Type == event.EventRedaction ||
				e.StateKey != nil ||
				e.Unsigned.RedactedBecause != nil {
				atomic.AddUint32(&purgedCount, ^uint32(0))
				return
			}
			if _, err := f.Client.RedactEvent(roomID, e.ID, mautrix.ReqRedact{}); err != nil {
				log.Println(err)
			}
		}(*e)
	}
	wg.Wait()

	if resp.End == "" {
		return nil // no more messages
	}

	// Recurse until we reach the end
	if u := int(purgedCount); u < limit {
		miss := limit - u
		return f.PurgeMessages(roomID, resp.End, miss)
	}

	return nil
}
