// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"strconv"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// The max amount of messages to fetch at once—the server will only give about
// ~1000 events.
const fetchLimit = 1000

// RedactMessage only redacts message events, skipping redaction events, already
// redacted events, and state events.
func RedactMessage(ev event.Event) (err error) {
	if ev.StateKey != nil {
		return
	}

	if ev.Type != event.EventRedaction && ev.Unsigned.RedactedBecause == nil {
		<-limit
		_, err = Client.RedactEvent(ev.RoomID, ev.ID, mautrix.ReqRedact{})
		return
	}
	return
}

func redactMessage(ev event.Event) {
	if err := RedactMessage(ev); err != nil {
		log.Println(err)
	}
}

func validate(resp *mautrix.RespMessages, err error) (*mautrix.RespMessages, error) {
	if err == nil && resp == nil {
		return resp, errNilMsgResponse
	}
	return resp, err
}

// PurgeUser redacts optionally a limit or all messages sent by a specified
// user. This is implemented efficiently using a filter to only obtain the
// events sent by the user.
func PurgeUser(body []string, ev event.Event) {
	user := id.UserID(body[0])

	var max int
	var limit bool
	if len(body) > 1 {
		i, err := strconv.Atoi(body[1])
		if err != nil {
			sendNotice(ev.RoomID, "not a valid integer of messages to purge")
			return
		}
		max = i
		limit = true
	}

	filter := userFilter(user)
	msg, err := validate(Client.Messages(ev.RoomID, "", "", 'b', &filter, fetchLimit))

	var prev string
	var i int
	for err == nil && msg.End != prev {
		prev = msg.End
		for _, e := range msg.Chunk {
			if limit {
				if i >= max {
					sendNotice(ev.RoomID, "Purging messages done!")
					return
				}
				i++
			}
			go redactMessage(*e)
		}
		msg, err = validate(Client.Messages(ev.RoomID, msg.End, "", 'b', &filter, fetchLimit))
	}
	log.Println("purging user messages failed with", err)
}

// PurgeMessages redacts all message events newer than the specified event ID.
// It's loosely inspired by Telegram's SophieBot mechanics.
func PurgeMessages(body []string, ev event.Event) {
	relate := ev.Content.AsMessage().RelatesTo
	if relate == nil {
		sendNotice(ev.RoomID, "Reply to the message you want to purge!")
		return
	}

	c, err := Client.Context(ev.RoomID, relate.EventID, purgeFilter, 1)
	if err != nil {
		log.Println("fetching context failed with error", err)
		return
	}
	go RedactMessage(*c.Event)

	msg, err := validate(Client.Messages(ev.RoomID, c.End, "", 'f', purgeFilter, fetchLimit))
	if msg != nil {
		msg.Chunk = append(c.EventsAfter, msg.Chunk...)
	}

	for err == nil {
		for _, e := range msg.Chunk {
			go redactMessage(*e)
			if e.ID == ev.ID {
				sendNotice(ev.RoomID, "Purging messages done!")
				return
			}
		}
		msg, err = validate(Client.Messages(ev.RoomID, msg.End, "", 'f', purgeFilter, fetchLimit))
	}
	log.Println("fetching messages failed with", err)
}

// CommandPurge is a simple function to be invoked by the purge keyword.
func CommandPurge(body []string, ev event.Event) {
	if !hasPerms(ev.RoomID, event.EventRedaction) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	if len(body) > 0 {
		PurgeUser(body, ev)
		return
	}
	PurgeMessages(body, ev)
}

var (
	errNilMsgResponse = errors.New("/messages response was nil, server has nothing to send us")
)
