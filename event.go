// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"bufio"
	"log"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

// isUnreadable returns whether a line is prefixed with an unreadable constant.
func isUnreadable(r byte) bool {
	switch r {
	case '*', '>':
		return true
	}
	return false
}

// HandleUserPolicy handles m.policy.rule.user events by banning literals and
// glob banning globs.
func HandleUserPolicy(s mautrix.EventSource, ev *event.Event) {
	if ev.Sender == Client.UserID {
		return
	}

	m := ev.Content.AsModPolicy()

	switch m.Recommendation {
	// TODO: remove non-spec mjolnir recommendation
	case "m.ban", "org.matrix.mjolnir.ban":
		if err := matchBan(options{User: m.Entity, RoomID: ev.RoomID}); err == errNotUser {
			sendNotice(ev.RoomID, "not a valid glob pattern!")
		} else if err != nil {
			log.Println(err)
		}
	}
}

// HandleServerPolicy handles m.policy.rule.server events. Initially limited to
// room admins but could possibly be extended to members of specific rooms.
func HandleServerPolicy(s mautrix.EventSource, ev *event.Event) {
	if ev.Sender == Client.UserID {
		return
	}

	m := ev.Content.AsModPolicy()
	switch m.Recommendation {
	// TODO: remove non-spec mjolnir recommendation
	case "m.ban", "org.matrix.mjolnir.ban":
		if err := BanServer(ev.RoomID, m.Entity); err != nil {
			log.Println(err)
		}
	}
}

// HandleMember handles `m.room.member` events.
func HandleMember(s mautrix.EventSource, ev *event.Event) {
	m := ev.Content.AsMember()

	if welcome && isNewJoin(*ev) && s&mautrix.EventSourceTimeline > 0 {
		display, sender, room := m.Displayname, ev.Sender, ev.RoomID
		if err := WelcomeMember(display, sender, room); err != nil {
			log.Println(err)
		}
	}
}

// HandleMessage handles m.room.message events.
func HandleMessage(s mautrix.EventSource, ev *event.Event) {
	if ev.Sender == Client.UserID {
		return
	}

	body := ev.Content.AsMessage().Body

	// var once sync.Once

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 1 || isUnreadable(line[0]) {
			continue
		}
		/*
			if l := strings.ToLower(line); firefox && strings.Contains(l, "firefox") {
				once.Do(func() {
					if err := SendFallacy(ev.RoomID); err != nil {
						log.Println(err)
					}
				})
			}
		*/
		if !strings.EqualFold("!fallacy", fields[0]) {
			continue
		}
		notifyListeners(fields, *ev)
	}
}

// HandleTombStone handles m.room.tombstone events, automatically joining the
// new room.
func HandleTombstone(_ mautrix.EventSource, ev *event.Event) {
	var (
		room   = ev.Content.Raw["replacement_room"].(string)
		reason = map[string]string{"reason": "following room upgrade"}
	)
	if _, err := Client.JoinRoom(room, "", reason); err != nil {
		sendNotice(ev.RoomID, "attempting to join room", room, "failed with error:", err.Error())
	}
}
