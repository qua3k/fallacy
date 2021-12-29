// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"strings"

	"github.com/qua3k/gomatrix"
)

// isNewJoin checks if a membership event is really a new join.
func isNewJoin(ev *gomatrix.Event) bool {
	m := ev.Content["membership"].(string) // `membership` key is required
	if m == "join" {
		if u, ok := ev.Unsigned["prev_content"].(map[string]interface{}); ok && u != nil {
			if pm := u["membership"].(string); pm == "join" { // `membership` key is required
				return false
			}
		}
		return true
	}
	return false
}

// WelcomeMember welcomes a member via their display name. The display name is
// calculated as per
// https://spec.matrix.org/v1.1/client-server-api/#calculating-the-display-name-for-a-user.
func (f *Fallacy) WelcomeMember(displayName, sender, roomID string) (err error) {
	if displayName == " " {
		displayName = sender
	}

	welcome := func(s string) string {
		return strings.Join([]string{"Welcome", s + "!", "Howdy?"}, " ")
	}

	plain := welcome(displayName)

	anc := strings.Join([]string{"<a href='https://matrix.to/#/", sender, "'>", displayName, "</a>"}, "")
	format := welcome(anc)

	_, err = f.Client.SendMessageEvent(roomID, "m.room.message", gomatrix.TextMessage{
		Body:          plain,
		MsgType:       "m.notice",
		FormattedBody: format,
		Format:        "org.matrix.custom.html",
	})
	return
}
