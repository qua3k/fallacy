// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"strings"

	"github.com/qua3k/gomatrix"
)

// IsDisplayNameOrAvatar: check if a membership event is a display name/avatar
// change
func IsDisplayOrAvatar(ev *gomatrix.Event) bool {
	m := ev.Content["membership"].(string) // required
	u := ev.Unsigned["prev_content"].(map[string]interface{})
	if u != nil {
		pm := u["membership"].(string) // required
		if m == "join" && pm == "join" {
			return true
		}
	}
	return false
}

// WelcomeMember: welcome a member via their display name or mxid
func (f *FallacyClient) WelcomeMember(displayName, sender, roomID string) (err error) {
	name := sender
	if displayName != "" {
		name = displayName
	}
	m := strings.Join([]string{"Welcome", name + "!", "Howdy?"}, " ")
	_, err = f.Client.SendNotice(roomID, m)
	return
}
