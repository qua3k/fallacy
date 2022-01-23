// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// fallacy provides wrappers around the less-pretty mautrix/go package. It's
// mainly meant for fallacy bot use but can be imported to other packages.

package fallacy

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const usage = `this is how you use the bot:
ban [MEMBER] [REASON]				bans your enemies
mute [MEMBER]						avoid having to hear your friends
purge [NUMBER]						gets rid of your shitposts
welcome [BOOL]						welcomes new members`

// The fallacy stickers we can use.
var fallacyStickers = [...]id.ContentURI{
	{
		Homeserver: "spitetech.com",
		FileID:     "XFgJMFCXulNthUiFUDqoEzuD",
	},
	{
		Homeserver: "spitetech.com",
		FileID:     "rpDChtvmojnErFdIZgfKktJW",
	},
	{
		Homeserver: "spitetech.com",
		FileID:     "KLJKMzTyTYKiHdHKSYKtNVXb",
	},
	{
		Homeserver: "spitetech.com",
		FileID:     "EdDSfNluLxYOfJmFKTDSXmaG",
	},
	{
		Homeserver: "spitetech.com",
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
	Handlers map[string][]commandListener
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
func NewFallacy(homeserverURL, userID, accessToken string, config *Config) (Fallacy, error) {
	cli, err := mautrix.NewClient(homeserverURL, id.UserID(userID), accessToken)
	if err != nil {
		return Fallacy{}, err
	}

	return Fallacy{
		Client: cli,
		Config: config,
	}, nil
}

// printHelp sends the help message into a room, propagating errors from SendNotice.
func (f *Fallacy) printHelp(roomID id.RoomID) (err error) {
	_, err = f.Client.SendNotice(roomID, usage)
	return
}

// SendFallacy sends a random fallacy into the chat. Users of this should
// explicitly call rand.Seed().
func (f *Fallacy) SendFallacy(ev event.Event) (err error) {
	const DefaultStickerSize = 256

	length := len(fallacyStickers)
	if length == 0 {
		return
	}

	i := rand.Intn(length)
	url := fallacyStickers[i]
	_, err = f.Client.SendMessageEvent(ev.RoomID, event.EventSticker, &event.MessageEventContent{
		Body: "no firefox here",
		Info: &event.FileInfo{
			Height: DefaultStickerSize,
			ThumbnailInfo: &event.FileInfo{
				Height: DefaultStickerSize,
				Width:  DefaultStickerSize,
			},
			ThumbnailURL: url.CUString(),
			Width:        DefaultStickerSize,
		},
		URL: url.CUString(),
	})
	return
}

// HandleUserPolicy handles m.policy.rule.user events. Initially limited to
// room admins but could possibly be extended to specific rooms.
func (f *Fallacy) HandleUserPolicy(_ mautrix.EventSource, ev *event.Event) {
	if ev.Sender == f.Client.UserID || !f.isAdmin(ev.RoomID, ev.Sender) {
		return
	}

	r, ok := ev.Content.Raw["recommendation"].(string)
	if !ok {
		log.Printf("asserting `recommendation` key failed! expected string, got: %T\n", r)
		return
	}

	switch r {
	case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove non-spec mjolnir recommendation
		e, ok := ev.Content.Raw["entity"].(string)
		if !ok {
			log.Printf("asserting `entity` key failed! expected string, got: %T\n", e)
			return
		}

		g, err := glob.Compile(e)
		if err != nil {
			f.attemptSendNotice(ev.RoomID, "not a valid glob pattern!")
			return
		}
		f.GlobBanJoinedRooms(g)
	}
}

// HandleServerPolicy handles m.policy.rule.server events. Initially limited to
// room admins but could possibly be extended to specific rooms.
func (f *Fallacy) HandleServerPolicy(_ mautrix.EventSource, ev *event.Event) {
	if ev.Sender == f.Client.UserID || !f.isAdmin(ev.RoomID, ev.Sender) {
		return
	}

	r, ok := ev.Content.Raw["recommendation"].(string)
	if !ok {
		log.Printf("asserting `recommendation` key failed! expected string, got: %T\n", r)
		return
	}

	switch r {
	case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove non-spec mjolnir recommendation
		e, ok := ev.Content.Raw["entity"].(string)
		if !ok {
			log.Printf("asserting `entity` key failed! expected string, got: %T\n", e)
			return
		}
		f.BanServerJoinedRooms(e)
	}
}

// HandleMember handles `m.room.member` events.
func (f *Fallacy) HandleMember(_ mautrix.EventSource, ev *event.Event) {
	sender, room := ev.Sender, ev.RoomID
	if err := ev.Content.ParseRaw(event.StateMember); err != nil {
		log.Println("parsing member event failed with:", err)
		return
	}

	mem := ev.Content.AsMember()
	if mem == nil {
		return
	}

	if f.Config.Welcome && isNewJoin(*ev) {
		display := mem.Displayname
		f.WelcomeMember(display, sender, room)
	}
}

// HandleMessage handles m.room.message events.
func (f *Fallacy) HandleMessage(ev *event.Event) {
	if ev.Sender == f.Client.UserID {
		return
	}

	if err := ev.Content.ParseRaw(event.EventMessage); err != nil {
		log.Println("parsing message event failed with:", err)
	}

	b := ev.Content.AsMessage().Body
	l := strings.ToLower(b)

	if f.Config.Firefox {
		if l := strings.ToLower(b); strings.Contains(l, "firefox") || strings.Contains(l, "fallacy") {
			f.sendFallacy(ev.RoomID)
			return
		}
	}

	if !strings.HasPrefix(l, "!"+f.Config.Name) {
		return
	}

	// IMPORTANT: All commands are gated under the admin permission check.
	if f.isAdmin(ev.RoomID, ev.Sender) {
		s := strings.Fields(b)
		if len(s) < 3 {
			f.printHelp(ev.RoomID)
			return
		}

	}

	s := strings.Fields(b)
	if len(s) < 3 {
		f.printHelp(ev.RoomID)
		return
	}

	switch s[1] {
	case "mute":
		if err := f.MuteUser(ev.RoomID, ev.Sender, s[2]); err != nil {
			log.Println(err)
		}
		return
	case "purge":
		l, err := strconv.Atoi(s[2])
		if err != nil {
			f.printHelp(ev.RoomID)
			return
		}
		if err := f.PurgeMessages(ev.RoomID, "", l); err != nil {
			log.Println(err)
		}
		return
	case "unmute":
		if err := f.UnmuteUser(ev.RoomID, ev.Sender, s[2]); err != nil {
			log.Println(err)
		}
		return
	}

	// messages := make(chan string)
}

// HandleTombStone handles m.room.tombstone events, automatically joining the
// new room.
func (f *Fallacy) HandleTombstone(_ mautrix.EventSource, ev *event.Event) {
	r, ok := ev.Content.Raw["replacement_room"].(string)
	if !ok {
		log.Printf("asserting `replacement_room` key failed! expected string, got: %T\n", r)
		return
	}

	reason := map[string]string{"reason": "following room upgrade"}
	if _, err := f.Client.JoinRoom(r, "", reason); err != nil {
		msg := strings.Join([]string{"attempting to join room", r, "failed with error:", err.Error()}, " ")
		f.attemptSendNotice(ev.RoomID, msg)
	}
}
