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
func isDisplayOrAvatar(ev *gomatrix.Event) bool {
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
func (f *Fallacy) WelcomeMember(displayName, sender, roomID string) (err error) {
	nickname := sender
	if displayName != " " {
		nickname = displayName
	}
	anc := strings.Join([]string{"<a href='https://matrix.to/#/", sender, "'>", nickname, "</a>"}, "")

	plain := strings.Join([]string{"Welcome", nickname + "!", "Howdy?"}, " ")
	format := strings.Join([]string{"Welcome", anc + "!", "Howdy?"}, " ")

	_, err = f.Client.SendMessageEvent(roomID, "m.room.message", gomatrix.TextMessage{
		Body:          plain,
		MsgType:       "m.notice",
		FormattedBody: format,
		Format:        "org.matrix.custom.html",
	})
	return
}
