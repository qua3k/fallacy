// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// RedactMessage only redacts message events, skipping redaction events, already
// redacted events, and state events.
func (f *Fallacy) RedactMessage(ev event.Event) (err error) {
	if ev.StateKey != nil {
		return
	}

	if ev.Type != event.EventRedaction && ev.Unsigned.RedactedBecause == nil {
		_, err = f.Client.RedactEvent(ev.RoomID, ev.ID, mautrix.ReqRedact{})
	}
	return
}

func (f *Fallacy) purgeEvents(evs []*event.Event) {
	for _, e := range evs {
		if e == nil {
			continue
		}
		go f.RedactMessage(*e)
	}
}

func (f *Fallacy) redactUsers(users []string, ev event.Event) {
	for _, u := range users {
		if id.UserID(u) != ev.Sender {
			continue
		}
		go f.RedactMessage(ev)
	}
}

// PurgeUsers redacts all messages sent by the specified users.
func (f *Fallacy) PurgeUsers(users []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.EventRedaction) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	filter := purgeUserFilter(users)
	msg, err := f.Client.Messages(ev.RoomID, "", "", 'b', &filter, maxFetchLimit)
	if err != nil {
		log.Println(err)
		return
	}

	var prevEnd string
	for msg.End != prevEnd {
		prevEnd = msg.End
		for _, e := range msg.Chunk {
			if e == nil {
				continue
			}
			go f.redactUsers(users, *e)
		}
		msg, err = f.Client.Messages(ev.RoomID, msg.End, "", 'b', &filter, maxFetchLimit)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

// PurgeMessages redacts all message events newer than the specified event ID.
// It's loosely inspired by Telegram's SophieBot mechanics.
func (f *Fallacy) PurgeMessages(_ []string, ev event.Event) {
	if !f.hasPerms(ev.RoomID, event.EventRedaction) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	relatesTo := ev.Content.AsMessage().RelatesTo
	if relatesTo == nil {
		f.attemptSendNotice(ev.RoomID, "Reply to the message you want to purge!")
		return
	}

	c, err := f.Client.Context(ev.RoomID, relatesTo.EventID, &purgeFilter, 1)
	if err != nil {
		log.Println(err)
		return
	}

	if c.Event == nil {
		log.Println("event is nil (should this happen)?")
		return
	}
	go f.RedactMessage(*c.Event)
	go f.purgeEvents(c.EventsAfter)

	msg, err := f.Client.Messages(ev.RoomID, c.End, "", 'f', &purgeFilter, maxFetchLimit)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		for _, e := range msg.Chunk {
			if e == nil {
				continue
			}
			go f.RedactMessage(*e)
			if e.ID == ev.ID {
				return
			}
		}
		msg, err = f.Client.Messages(ev.RoomID, msg.End, "", 'f', &purgeFilter, maxFetchLimit)
		if err != nil {
			log.Println(err)
			return
		}
	}
}
