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
		handleLimit(0, func() error {
			_, err = Client.RedactEvent(ev.RoomID, ev.ID, mautrix.ReqRedact{})
			return err
		})
	}
	return
}

func redactUser(user id.UserID, ev event.Event) (err error) {
	if ev.Sender != user {
		return nil
	}
	return RedactMessage(ev)
}

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

func doMessages(resp *mautrix.RespMessages, err error) (*mautrix.RespMessages, error) {
	if resp == nil {
		return resp, errors.New("/messages response was nil, server has nothing to send us")
	}
	return resp, err
}

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
	msg, err := doMessages(Client.Messages(ev.RoomID, "", "", 'f', &filter, fetchLimit))
	if err != nil {
		log.Println(err)
	}

	var prev string
	var i int
	for msg.End != prev {
		prev = msg.End
		for _, e := range msg.Chunk {
			if limit {
				if i >= max {
					sendNotice(ev.RoomID, "Purging messages done!")
					return
				}
				i++
			}
			go func(e event.Event) {
				if err := redactUser(user, e); err != nil {
					log.Println(err)
				}
			}(*e)
		}
		if msg, err = doMessages(Client.Messages(ev.RoomID, "", "", 'f', &filter, fetchLimit)); err != nil {
			log.Println(err)
			return
		}
	}
}

// PurgeMessages redacts all message events newer than the specified event ID.
// It's loosely inspired by Telegram's SophieBot mechanics.
func PurgeMessages(body []string, ev event.Event) {
	relate := ev.Content.AsMessage().RelatesTo
	if relate == nil {
		sendNotice(ev.RoomID, "Reply to the message you want to purge!")
		return
	}

	c, err := Client.Context(ev.RoomID, relate.EventID, &purgeFilter, 1)
	if err != nil {
		log.Println("fetching context failed with error", err)
		return
	}
	go RedactMessage(*c.Event)

	msg, err := doMessages(Client.Messages(ev.RoomID, c.End, "", 'f', &purgeFilter, fetchLimit))
	if err != nil {
		log.Println("fetching messages failed with error:", err)
		return
	}

	msg.Chunk = append(c.EventsAfter, msg.Chunk...)
	for {
		for _, e := range msg.Chunk {
			go func(e event.Event) {
				if err := RedactMessage(e); err != nil {
					log.Println(err)
				}
			}(*e)
			if e.ID == ev.ID {
				sendNotice(ev.RoomID, "Purging messages done!")
				return
			}
		}
		msg, err = doMessages(Client.Messages(ev.RoomID, msg.End, "", 'f', &purgeFilter, fetchLimit))
		if err != nil {
			log.Println("fetching context failed with error", err)
			return
		}
	}
}
