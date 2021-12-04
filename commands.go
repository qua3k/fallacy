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
	level, ok := event.(int)
	if !ok {
		level = 50
	}
	return
}

// GetMessageLevel Returns the level users are required to have to send
// messages.
func GetMessageLevel(content map[string]interface{}) (level int) {
	level = 50
	if d, ok := content["events_default"].(int); d != 0 && ok {
		level = d
	}

	if pls, ok := content["events"].(map[string]int); ok {
		if m, ok := pls["m.room.message"]; m != 0 && ok {
			level = m
		}
	}

	return
}

// userCanMute: determine if the user is allowed to mute someone by checking
// ban/kick/redact permissions
func (f *FallacyClient) userCanMute(pl map[string]interface{}, userID string) bool {
	bp, kp, rp := modPower(pl["ban"]), modPower(pl["kick"]), modPower(pl["redact"])
	p := []int{kp, bp, rp}

	min := p[0]
	for _, value := range p {
		if value < min {
			min = value
		}
	}

	var usrPwr int
	if usrDef, ok := pl["users_default"].(int); ok {
		usrPwr = usrDef
	}
	if usrs, ok := pl["users"].(map[string]int); ok {
		if pwr := usrs[userID]; pwr != 0 {
			usrPwr = pwr
		}
	}

	if usrPwr >= min {
		return true
	}

	return false
}

// secret
func (f *FallacyClient) spiteTech(body, roomID string) {
	if strings.Contains(body, "firefox") {
		f.Client.SendSticker(roomID, "👨 (man)", "mxc://spitetech.com/XFgJMFCXulNthUiFUDqoEzuD")
	}
}

// Mutes a user in a specific room with their MXID.
// It fetches a power levels event, then searches for the power level members are
// allowed to send messages, first by the `events_default` key and then the
// `m.room.message` key of `events`.
func (f *FallacyClient) MuteUser(roomID, senderID, targetID string) (err error) {
	c, err := f.Client.LookUpStateEvent("m.room.power_levels", roomID, "")
	if err == nil {
		if !f.userCanMute(c, senderID) {
			return errors.New("user not authorized to mute")
		}
		level := GetMessageLevel(c)
		if u, ok := c["users"].(map[string]int); ok {
			u[targetID] = level - 1
		}
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", c)
	}

	return
}

// Unmutes a user in a specific room with their MXID.
// It fetches a power levels event, then searches for the power level members are
// allowed to send messages, first by the `events_default` key and then the
// `m.room.message` key of `events`.
func (f *FallacyClient) UnmuteUser(roomID, senderID, targetID string) (err error) {
	c, err := f.Client.LookUpStateEvent("m.room.power_levels", roomID, "")
	if err == nil {
		if !f.userCanMute(c, senderID) {
			return errors.New("user not authorized to unmute")
		}
		level := GetMessageLevel(c)
		if u, ok := c["users"].(map[string]int); ok {
			u[targetID] = level + 1
		}
		_, err = f.Client.SendStateEvent(roomID, "m.room.power_levels", "", c)
	}

	return
}

// PurgeMessages: redact a number of room events in a room, optionally ending at
// a specific message.
func (f *FallacyClient) PurgeMessages(roomID, end string, limit int) error {
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
