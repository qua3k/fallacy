// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"
	"strings"

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
	if pc == nil {
		return true
	}

	if err := pc.ParseRaw(event.StateMember); err != nil {
		log.Println("parsing member event failed with:", err)
		return false
	}

	if p := pc.AsMember(); isJoin(p.Membership) {
		return false
	}
	return true
}

// WelcomeMember welcomes a member via their display name. The display name is
// calculated as per
// https://spec.matrix.org/v1.1/client-server-api/#calculating-the-display-name-for-a-user.
func (f *Fallacy) WelcomeMember(displayName string, sender id.UserID, roomID id.RoomID) (err error) {
	senderString := sender.String()
	switch displayName {
	case "", " ":
		displayName = senderString
	}

	welcome := func(s string) string {
		return strings.Join([]string{"Welcome", s + "!", "Howdy?"}, " ")
	}

	anchor := strings.Join([]string{"<a href='https://matrix.to/#/", senderString, "'>", displayName, "</a>"}, "")
	_, err = f.Client.SendMessageEvent(roomID, event.EventMessage, event.MessageEventContent{
		Body:          welcome(displayName),
		Format:        event.FormatHTML,
		FormattedBody: welcome(anchor),
		MsgType:       event.MsgNotice,
	})
	return
}
