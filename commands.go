// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"github.com/qua3k/gomatrix"
)

const MinimumFetch int = 20 // the minimum number of messages to fetch during a request to `/messages`

// modPower returns the default mod power for many matrix events
func modPower(event int) int {
	if event == 0 {
		return 50
	}
	return event
}

// getMessageLevel returns the level users are required to have to send by
// checking both the `events_default` key and the `m.room.message` key of
// the `events` object
func getMessageLevel(pwr *gomatrix.RespPowerLevels) (level int) {
	if ed := pwr.EventsDefault; ed != 0 { // `events_default` defaults to zero
		level = ed
	}
	if m, ok := pwr.Events["m.room.mesage"]; ok {
		level = m
	}
	return
}

// userCanMute determines if the specified user has the necessary permissions
// to mute another user in the room by checking kick/ban/redact power levels.
func (f *Fallacy) userCanMute(pwr *gomatrix.RespPowerLevels, userID string) bool {
	var usrPwr int

	cp := func(i int) {
		if i != 0 {
			usrPwr = i
		}
	}
	cp(pwr.UsersDefault)
	cp(pwr.Users[userID])

	minInt := func(i ...int) (m int) {
		m = i[0]
		for _, v := range i {
			if v < m {
				m = v
			}
		}
		return
	}

	bp, kp, rp := modPower(pwr.Ban), modPower(pwr.Kick), modPower(pwr.Redact) // what if the power level is 0?
	return usrPwr >= minInt(kp, bp, rp)
}

// MuteUser mutes a target user in a specified room.
func (f *Fallacy) MuteUser(roomID, senderID, targetID string) (err error) {
	pwr, err := f.Client.PowerLevels(roomID)
	if err == nil {
		if !f.userCanMute(pwr, senderID) {
			return errors.New("user not authorized to mute")
		}
		level := getMessageLevel(pwr) - 1
		if pwr.Users[targetID] < level {
			return errors.New("cannot mute a user that is already muted")
		} else if level == pwr.EventsDefault {
			delete(pwr.Users, targetID)
		} else {
			pwr.Users[targetID] = level
		}
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", pwr)
	}
	return
}

// UnmuteUser unmutes a target user in a specified room.
func (f *Fallacy) UnmuteUser(roomID, senderID, targetID string) (err error) {
	pwr, err := f.Client.PowerLevels(roomID)
	if err == nil {
		if !f.userCanMute(pwr, senderID) {
			return errors.New("user not authorized to mute")
		}
		level := getMessageLevel(pwr)
		if pwr.Users[targetID] >= level {
			log.Println("working!")
			return errors.New("cannot unmute a user that is already unmuted")
		} else if level == pwr.EventsDefault {
			delete(pwr.Users, targetID)
		} else {
			pwr.Users[targetID] = level
		}
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", pwr)
	}
	return
}

// PurgeMessages redacts a number of room events in a room, optionally ending at
// a specific message token obtained from a previous request to the endpoint.
func (f *Fallacy) PurgeMessages(roomID, end string, limit int) error {
	fetch := limit
	if fetch < MinimumFetch {
		fetch = MinimumFetch // always fetch a minimum amount of messages
	}

	resp, err := f.Client.Messages(roomID, "", end, "", 'b', fetch)
	if err != nil {
		return err
	}

	purged := uint32(limit)

	if limit > len(resp.Chunk) {
		limit = len(resp.Chunk) // delete all the events we can
	}

	var wg sync.WaitGroup
	for _, e := range resp.Chunk[0:limit] { // slice the necessary elements
		wg.Add(1)
		go func(e gomatrix.Event) {
			defer wg.Done()
			_, ok := e.Unsigned["redacted_because"].(map[string]interface{})

			if e.Type == "m.room.redaction" ||
				e.StateKey != nil ||
				ok {
				atomic.AddUint32(&purged, ^uint32(0)) // skip over `m.room.redactions`, redacted events, and state events
				return
			}

			if _, err := f.Client.RedactEvent(roomID, e.ID, &gomatrix.ReqRedact{}); err != nil {
				log.Println(err)
			}
		}(e)
	}
	wg.Wait()

	if resp.End == "" {
		return nil // no more messages
	}

	// Recurse until we reach the end
	if u := int(purged); u < limit {
		miss := limit - u
		return f.PurgeMessages(roomID, resp.End, miss)
	}

	return nil
}
