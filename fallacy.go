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

const usage = `# fallacy bot help
## the following commands are available:

BAN:                        bans your enemies
MUTE:                       avoid having to hear your friends
PURGE:                      gets rid of your shitposts
WELCOME:                    welcomes new members`

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
	Handlers map[string][]CallbackStruct
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
		Handlers: make(map[string][]CallbackStruct),
	}, nil
}

// unreadable are the unreadable constants.
var unreadable = [2]string{"*", ">"}

// isUnreadable returns whether a line is prefixed with an unreadable constant.
func isUnreadable(line string) bool {
	for _, s := range unreadable {
		if strings.HasPrefix(line, s) {
			return true
		}
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
		if err := f.GlobBanJoinedRooms(g); err != nil {
			log.Println()
		}
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
func (f *Fallacy) HandleMember(s mautrix.EventSource, ev *event.Event) {
	mem := ev.Content.AsMember()
	if mem == nil {
		log.Println("HandleMember failed, got a nil pointer!")
		return
	}

	if f.Config.Welcome && isNewJoin(*ev) && s == mautrix.EventSourceTimeline {
		display, sender, room := mem.Displayname, ev.Sender, ev.RoomID
		f.WelcomeMember(display, sender, room)
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
