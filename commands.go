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

// minInt is a variadic function to find the minimum of integers
func minInt(i ...int) (m int) {
	m = i[0]
	for _, v := range i {
		if v < m {
			m = v
		}
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
// a specific message.
func (f *Fallacy) PurgeMessages(roomID, end string, limit int32) error {
	resp, err := f.Client.Messages(roomID, "", end, "", 'b', int(limit))
	if err != nil {
		return err
	}
	if resp.End == "" {
		return nil // no more messages
	}

	var wg sync.WaitGroup
	count := int32(len(resp.Chunk))

	for _, e := range resp.Chunk {
		wg.Add(1)
		go func(e gomatrix.Event) {
			defer wg.Done()
			_, ok := e.Unsigned["redacted_because"].(map[string]interface{})
			if e.Type == "m.room.redaction" || ok || e.StateKey != nil {
				atomic.AddInt32(&count, ^int32(0))
				return
			}
			if _, err := f.Client.RedactEvent(roomID, e.ID, &gomatrix.ReqRedact{}); err != nil {
				log.Println(err)
			}
		}(e)
	}
	wg.Wait()

	// Recurse until we reach the end
	if count < limit {
		miss := limit - count
		err := f.PurgeMessages(roomID, resp.End, miss)
		return err
	}

	return nil
}
