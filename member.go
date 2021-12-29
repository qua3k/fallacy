// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"github.com/qua3k/gomatrix"
)

// isNewJoin checks if a membership event is really a new join.
func isNewJoin(ev *gomatrix.Event) bool {
	m, ok := ev.Content["membership"].(string)

	join := func(m string, ok bool) bool {
		return m == "join" && ok
	}

	if join(m, ok) { // `membership` key must be string
		if u, ok := ev.Unsigned["prev_content"].(map[string]interface{}); ok {
			if pm, ok := u["membership"].(string); join(pm, ok) { // `membership` key must be string
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
		return "Welcome " + s + "! Howdy?"
	}

	anc := "<a href='https://matrix.to/#/" + sender + "'>" + displayName + "</a>"
	plain := welcome(displayName)
	format := welcome(anc)

	_, err = f.Client.SendMessageEvent(roomID, "m.room.message", gomatrix.TextMessage{
		Body:          plain,
		MsgType:       "m.notice",
		FormattedBody: format,
		Format:        "org.matrix.custom.html",
	})
	return
}
