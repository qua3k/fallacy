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
func modPower(event interface{}) (level int) {
	var ok bool
	if level, ok = event.(int); !ok {
		level = 50
	}
	return
}

// getMessageLevel: returns the level users are required to have to send
// messages.
func getMessageLevel(content map[string]interface{}) (level int) {
	if d, ok := content["events_default"].(int); ok {
		level = d
	}

	if pls, ok := content["events"].(map[string]int); ok {
		if m, ok := pls["m.room.message"]; ok {
			level = m
		}
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
func (f *Fallacy) userCanMute(pls map[string]interface{}, userID string) bool {
	var usrPwr int
	if usrDef, ok := pls["users_default"].(int); ok {
		usrPwr = usrDef
	}
	if usrs, ok := pls["users"].(map[string]int); ok {
		if pwr, ok := usrs[userID]; ok {
			usrPwr = pwr
		}
	}

	bp, kp, rp := modPower(pls["ban"]), modPower(pls["kick"]), modPower(pls["redact"])
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
	pls, err := f.Client.LookUpStateEvent("m.room.power_levels", roomID, "")
	if err == nil {
		if !f.userCanMute(pls, senderID) {
			return errors.New("user not authorized to mute")
		}
		level := getMessageLevel(pls)
		if u, ok := pls["users"].(map[string]int); ok { // figure out how to add it ourselves when it doesn't exist
			u[targetID] = level - 1
		}
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", pls)
	}

	return
}

// Unmutes a user in a specific room with their MXID.
// It fetches a power levels event, then searches for the power level members are
// allowed to send messages, first by the `events_default` key and then the
// `m.room.message` key of `events`.
func (f *Fallacy) UnmuteUser(roomID, senderID, targetID string) (err error) {
	pls, err := f.Client.LookUpStateEvent("m.room.power_levels", roomID, "")
	if err == nil {
		if !f.userCanMute(pls, senderID) {
			return errors.New("user not authorized to unmute")
		}
		level := getMessageLevel(pls)
		if u, ok := pls["users"].(map[string]int); ok {
			u[targetID] = level + 1
		}
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", pls)
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
