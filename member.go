// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// isNewJoin checks if a membership event is really a new join.
func isNewJoin(ev event.Event) bool {
	isJoin := func(m event.Membership) bool {
		return m == event.MembershipJoin
	}

	if m := ev.Content.AsMember(); !isJoin(m.Membership) {
		return false
	}

	pc := ev.Unsigned.PrevContent
	if pc != nil {
		if err := pc.ParseRaw(event.StateMember); err != nil {
			log.Println("parsing member event failed with:", err)
			return false
		}
		if p := pc.AsMember(); isJoin(p.Membership) {
			return false
		}
	}

	return true
}

// WelcomeMember welcomes a member via their display name. The display name is
// calculated as per
// https://spec.matrix.org/v1.1/client-server-api/#calculating-the-display-name-for-a-user.
func (f *Fallacy) WelcomeMember(displayName, sender string, roomID id.RoomID) (err error) {
	switch displayName {
	case "", " ":
		displayName = sender
	}

	welcome := func(s string) string {
		return "Welcome " + s + "! Howdy?"
	}

	plain := welcome(displayName)

	anchor := "<a href='https://matrix.to/#/" + sender + "'>" + displayName + "</a>"
	format := welcome(anchor)

	_, err = f.Client.SendMessageEvent(roomID, event.EventMessage, event.MessageEventContent{
		Body:          plain,
		Format:        event.FormatHTML,
		FormattedBody: format,
		MsgType:       event.MsgNotice,
	})
	return
}
