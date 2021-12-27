// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"strings"

	"github.com/qua3k/gomatrix"
)

// modPower: the default mod power for many matrix events
func modPower(event int) int {
	if event == 0 {
		return 50
	}
	return event
}

// getMessageLevel: returns the level users are required to have to send
// messages.
func getMessageLevel(pwr *gomatrix.RespPowerLevels) (level int) {
	if ed := pwr.EventsDefault; ed != 0 {
		level = ed
	}
	if m, ok := pwr.Events["m.room.mesage"]; ok {
		level = m
	}
	return
}

// minInt: variadic function to find min of integers
func minInt(i ...int) (m int) {
	m = i[0]
	for _, v := range i {
		if v < m {
			m = v
		}
	}
	return
}

// userCanMute: determine if the user is allowed to mute someone by checking
// ban/kick/redact permissions
func (f *Fallacy) userCanMute(pls *gomatrix.RespPowerLevels, userID string) bool {
	var usrPwr int

	if d := pls.UsersDefault; d != 0 {
		usrPwr = d
	}
	if p := pls.Users[userID]; p != 0 {
		usrPwr = p
	}

	bp, kp, rp := modPower(pls.Ban), modPower(pls.Kick), modPower(pls.Redact) // what if the power level is 0?
	return usrPwr >= minInt(kp, bp, rp)
}

// secret
func (f *Fallacy) spiteTech(body, roomID string) {
	if strings.Contains(body, "firefox") {
		f.Client.SendSticker(roomID, "👨 (man)", "mxc://spitetech.com/XFgJMFCXulNthUiFUDqoEzuD")
	}
}

// Mutes a user in a specific room with their MXID.
// It fetches a power levels event, then searches for the power level members are
// allowed to send messages, first by the `events_default` key and then the
// `m.room.message` key of `events`.
func (f *Fallacy) MuteUser(roomID, senderID, targetID string) (err error) {
	pwr, err := f.Client.PowerLevels(roomID)
	if err == nil {
		if !f.userCanMute(pwr, senderID) {
			return errors.New("user not authorized to mute")
		}
		level := getMessageLevel(pwr)
		pwr.Users[targetID] = level - 1
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", *pwr)
	}
	return
}

// Unmutes a user in a specific room with their MXID.
// It fetches a power levels event, then searches for the power level members are
// allowed to send messages, first by the `events_default` key and then the
// `m.room.message` key of `events`.
func (f *Fallacy) UnmuteUser(roomID, senderID, targetID string) (err error) {
	pwr, err := f.Client.PowerLevels(roomID)
	if err == nil {
		if !f.userCanMute(pwr, senderID) {
			return errors.New("user not authorized to mute")
		}
		level := getMessageLevel(pwr)
		pwr.Users[targetID] = level + 1
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", *pwr)
	}
	return
}

// PurgeMessages: redact a number of room events in a room, optionally ending at
// a specific message.
func (f *Fallacy) PurgeMessages(roomID, end string, limit int) error {
	resp, err := f.Client.Messages(roomID, "", "", end, 'b', limit)
	if err != nil {
		return err
	}
	if resp.End == "" {
		return nil // no more messages
	}

	for _, e := range resp.Chunk {
		// TODO: figure out if this races
		go func(e gomatrix.Event) {
			if _, err := f.Client.RedactEvent(roomID, e.ID, &gomatrix.ReqRedact{}); err != nil {
				log.Println(err)
			}
		}(e)
	}

	// Recurse until we reach the end
	if count := len(resp.Chunk); count < limit {
		miss := limit - count
		if err := f.PurgeMessages(roomID, resp.Start, miss); err != nil {
			return err
		}
	}

	return nil
}
