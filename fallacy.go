// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// fallacy provides wrappers around the less-pretty mautrix/go package. It's
// mainly meant for fallacy bot use but can be imported to other packages.

package fallacy

import (
	"bufio"
	"log"
	"math/rand"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const stickerServer = "spitetech.com"
const usage = "Hey, check out the usage guide at https://github.com/qua3k/fallacy/blob/main/USAGE.md"

// The fallacy stickers we can use.
var stickers = [...]id.ContentURI{
	{
		Homeserver: stickerServer,
		FileID:     "XFgJMFCXulNthUiFUDqoEzuD",
	},
	{
		Homeserver: stickerServer,
		FileID:     "rpDChtvmojnErFdIZgfKktJW",
	},
	{
		Homeserver: stickerServer,
		FileID:     "KLJKMzTyTYKiHdHKSYKtNVXb",
	},
	{
		Homeserver: stickerServer,
		FileID:     "EdDSfNluLxYOfJmFKTDSXmaG",
	},
	{
		Homeserver: stickerServer,
		FileID:     "ziTJliFmgUpxCTXgyjSMvNKA",
	},
}

// Configuration options for the bot.
type Config struct {
	Lock    sync.RWMutex
	Firefox bool     // should we harass firefox users
	Name    string   // the name of the bot
	Rules   []string // user rules; can be glob
	Welcome bool     // whether to welcome new members on join
}

// The main Fallacy struct containing the client and config.
type Fallacy struct {
	Client   *mautrix.Client
	Config   *Config
	Handlers map[string][]Callback
}

// NewConfig instantiates a new Config struct.
func NewConfig(firefox bool, name string, rules []string, welcome bool) Config {
	return Config{
		Firefox: firefox,
		Name:    name,
		Rules:   rules,
		Welcome: welcome,
	}
}

// NewFallacy instantiates a new Fallacy struct.
func NewFallacy(homeserverURL, userID, accessToken string, config *Config) (*Fallacy, error) {
	cli, err := mautrix.NewClient(homeserverURL, id.UserID(userID), accessToken)
	if err != nil {
		return &Fallacy{}, err
	}

	return &Fallacy{
		Client:   cli,
		Config:   config,
		Handlers: make(map[string][]Callback),
	}, nil
}

// isUnreadable returns whether a line is prefixed with an unreadable constant.
func isUnreadable(s byte) bool {
	switch s {
	case '*', '>':
		return true
	}
	return false
}

// SendFallacy sends a random fallacy into the chat. Users of this should
// explicitly call rand.Seed().
func SendFallacy(c *mautrix.Client, roomID id.RoomID) (err error) {
	const (
		defaultSize = 256
		length      = len(stickers)
	)

	u := stickers[rand.Intn(length)]
	_, err = c.SendMessageEvent(roomID, event.EventSticker, &event.MessageEventContent{
		Body: "no firefox here",
		Info: &event.FileInfo{
			Height: defaultSize,
			ThumbnailInfo: &event.FileInfo{
				Height: defaultSize,
				Width:  defaultSize,
			},
			ThumbnailURL: u.CUString(),
			Width:        defaultSize,
		},
		URL: u.CUString(),
	})
	return
}

// attemptSendNotice wraps Client.SendNotice, logging when a notice is unable to
// be sent.
func (f *Fallacy) attemptSendNotice(roomID id.RoomID, text string) *mautrix.RespSendEvent {
	if resp, err := f.Client.SendNotice(roomID, text); err == nil {
		return resp
	}
	msg := strings.Join([]string{"could not send notice", text, "into room", roomID.String()}, " ")
	log.Println(msg)
	return nil
}

// sendReply sends a message as a reply to another message.
func (f *Fallacy) sendReply(ev event.Event, s string) (*mautrix.RespSendEvent, error) {
	return f.Client.SendMessageEvent(ev.RoomID, event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    s,
		RelatesTo: &event.RelatesTo{
			Type:    event.RelReply,
			EventID: ev.ID,
		},
	})
}

// HandleUserPolicy handles m.policy.rule.user events. Initially limited to
// room admins but could possibly be extended to members of specific rooms.
func (f *Fallacy) HandleUserPolicy(_ mautrix.EventSource, ev *event.Event) {
	if ev.Sender == f.Client.UserID {
		return
	}

	switch ev.Content.Raw["recommendation"].(string) {
	case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove non-spec mjolnir recommendation
		g, err := glob.Compile(ev.Content.Raw["entity"].(string))
		if err != nil {
			f.attemptSendNotice(ev.RoomID, "not a valid glob pattern!")
			return
		}
		if err := f.GlobBanJoinedRooms(g); err != nil {
			log.Println(err)
		}
	}
}

// HandleServerPolicy handles m.policy.rule.server events. Initially limited to
// room admins but could possibly be extended to members of specific rooms.
func (f *Fallacy) HandleServerPolicy(_ mautrix.EventSource, ev *event.Event) {
	if ev.Sender == f.Client.UserID {
		return
	}

	switch ev.Content.Raw["recommendation"].(string) {
	case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove non-spec mjolnir recommendation
		e := ev.Content.Raw["entity"].(string)
		if err := f.BanServerJoinedRooms(e); err != nil {
			log.Println(err)
		}
	}
}

// HandleMember handles `m.room.member` events.
func (f *Fallacy) HandleMember(s mautrix.EventSource, ev *event.Event) {
	mem := ev.Content.AsMember()
	if mem == nil {
		log.Println("HandleMember failed, got a nil pointer!")
		return
	}

	tl := s & mautrix.EventSourceTimeline
	if f.Config.Welcome && isNewJoin(*ev) && tl > 0 {
		display, sender, room := mem.Displayname, ev.Sender, ev.RoomID
		if err := f.WelcomeMember(display, sender, room); err != nil {
			log.Println(err)
		}
	}
}

// HandleMessage handles m.room.message events.
func (f *Fallacy) HandleMessage(_ mautrix.EventSource, ev *event.Event) {
	if ev.Sender == f.Client.UserID {
		return
	}

	msg := ev.Content.AsMessage()
	if msg == nil {
		log.Println("HandleMessage failed, got a nil pointer!")
		return
	}

	body := msg.Body

	var once sync.Once

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 1 || isUnreadable(line[0]) {
			continue
		}

		if l := strings.ToLower(line); f.Config.Firefox && strings.Contains(l, "firefox") {
			once.Do(func() {
				if err := SendFallacy(f.Client, ev.RoomID); err != nil {
					log.Println(err)
				}
			})
		}

		if !strings.EqualFold("!"+f.Config.Name, fields[0]) {
			continue
		}
		f.notifyListeners(fields, *ev)
	}
}

// HandleTombStone handles m.room.tombstone events, automatically joining the
// new room.
func (f *Fallacy) HandleTombstone(_ mautrix.EventSource, ev *event.Event) {
	var (
		room   = ev.Content.Raw["replacement_room"].(string)
		reason = map[string]string{"reason": "following room upgrade"}
	)
	if _, err := f.Client.JoinRoom(room, "", reason); err != nil {
		msg := strings.Join([]string{"attempting to join room", room, "failed with error:", err.Error()}, " ")
		f.attemptSendNotice(ev.RoomID, msg)
	}
}
