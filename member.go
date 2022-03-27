// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// isNewJoin checks if a membership event is really a new join.
func isNewJoin(ev event.Event) bool {
	prev := ev.Unsigned.PrevContent

	if ev.Content.AsMember().Membership == event.MembershipJoin && prev == nil {
		return true
	}

	if m, ok := prev.Raw["membership"].(string); ok {
		if m == "join" {
			return false
		}
	}
	return true
}

// WelcomeMember welcomes a member via their display name. The display name is
// calculated as per
// https://spec.matrix.org/v1.1/Client-server-api/#calculating-the-display-name-for-a-user.
func WelcomeMember(display string, sender id.UserID, roomID id.RoomID) (err error) {
	senderStr := sender.String()

	// if the name is just whitespace we can just use sender ID
	if f := strings.Fields(display); len(f) == 0 {
		display = senderStr
	}

	welcome := func(s string) string {
		return strings.Join([]string{"Welcome", s + "!", "Howdy?"}, " ")
	}

	anchor := strings.Join([]string{"<a href='https://matrix.to/#/", senderStr, "'>", display, "</a>"}, "")
	_, err = Client.SendMessageEvent(roomID, event.EventMessage, event.MessageEventContent{
		Body:          welcome(display),
		Format:        event.FormatHTML,
		FormattedBody: welcome(anchor),
		MsgType:       event.MsgNotice,
	})
	return
}
