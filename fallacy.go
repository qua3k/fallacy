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

const usage = `fallacy bot help:
the following commands are available

*	ban (ban <glob>) — bans your enemies. Takes a glob.
*	mute (mute <mxid>) — avoid having to hear your friends.
*	pin — pins the message you replied to.
*	purge — deletes all messages from the message you replied to.
*	purgeuser (purgeuser <mxid>) — purges messages from that user until the beginning of time.
*	say (say <message>) — let the bot say something.
*	unmute (unmute <mxid>) — allow a peasant to speak.`

const stickerServer = "spitetech.com"

// The fallacy stickers we can use.
var fallacyStickers = [...]id.ContentURI{
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
func isUnreadable(line string) bool {
	switch {
	case strings.HasPrefix(line, "*"), strings.HasPrefix(line, ">"):
		return true
	}
	return false
}

// printHelp sends the help message into a room, propagating errors from SendNotice.
func (f *Fallacy) printHelp(roomID id.RoomID) (err error) {
	_, err = f.Client.SendNotice(roomID, usage)
	return
}

// SendFallacy sends a random fallacy into the chat. Users of this should
// explicitly call rand.Seed().
func (f *Fallacy) SendFallacy(roomID id.RoomID) (err error) {
	const defaultStickerSize = 256
	const length = len(fallacyStickers)

	i := rand.Intn(length)
	url := fallacyStickers[i]
	_, err = f.Client.SendMessageEvent(roomID, event.EventSticker, &event.MessageEventContent{
		Body: "no firefox here",
		Info: &event.FileInfo{
			Height: defaultStickerSize,
			ThumbnailInfo: &event.FileInfo{
				Height: defaultStickerSize,
				Width:  defaultStickerSize,
			},
			ThumbnailURL: url.CUString(),
			Width:        defaultStickerSize,
		},
		URL: url.CUString(),
	})
	return
}

// attemptSendNotice wraps Client.SendNotice, logging when a notice is unable to
// be sent.
func (f *Fallacy) attemptSendNotice(roomID id.RoomID, text string) {
	if _, err := f.Client.SendNotice(roomID, text); err == nil {
		return
	}
	msg := strings.Join([]string{"could not send notice", text, "into room", roomID.String()}, " ")
	log.Println(msg)
}

// HandleUserPolicy handles m.policy.rule.user events. Initially limited to
// room admins but could possibly be extended to members of specific rooms.
func (f *Fallacy) HandleUserPolicy(_ mautrix.EventSource, ev *event.Event) {
	if ev.Sender == f.Client.UserID {
		return
	}

	r := ev.Content.Raw["recommendation"].(string)

	switch r {
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

	r := ev.Content.Raw["recommendation"].(string)

	switch r {
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

	reader := strings.NewReader(body)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case len(line) < 1, isUnreadable(line):
			continue
		}

		if l := strings.ToLower(line); f.Config.Firefox && strings.Contains(l, "firefox") {
			if err := f.SendFallacy(ev.RoomID); err != nil {
				log.Println(err)
			}
		}

		fields, prefix := strings.Fields(line), "!"+f.Config.Name
		if !strings.EqualFold(prefix, fields[0]) {
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
