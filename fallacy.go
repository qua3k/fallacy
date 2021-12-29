// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// fallacy provides wrappers around the less-pretty gomatrix package. It's
// mainly meant for fallacy bot use but can be imported to other packages.

package fallacy

import (
	"log"
	"strconv"
	"strings"

	"github.com/qua3k/gomatrix"
)

const usage = `this is how you use the bot:
ban [MEMBER] [REASON]				bans your enemies
mute [MEMBER]						avoid having to hear your friends
purge [NUMBER]						gets rid of your shitposts
welcome [BOOL]						welcomes new members`

type Config struct {
	Firefox bool   // should we harass firefox users
	Name    string // the name of the bot
	Welcome bool   // whether to welcome new members on join
}

// The main Fallacy struct containing the configuration for the bot.
type Fallacy struct {
	Client *gomatrix.Client
	Config Config
}

// NewConfig instantiates a new Config struct.
func NewConfig(firefox bool, name string, welcome bool) Config {
	return Config{
		Firefox: firefox,
		Name:    name,
		Welcome: welcome,
	}
}

// NewFallacy instantiates a new Fallacy struct.
func NewFallacy(homeserverURL, userID, accessToken string, config Config) (Fallacy, error) {
	cli, err := gomatrix.NewClient(homeserverURL, userID, accessToken)
	if err != nil {
		return Fallacy{}, err
	}

	return Fallacy{
		Client: cli,
		Config: config,
	}, nil
}

// printHelp sends the help message into a room.
func (f *Fallacy) printHelp(roomID string) {
	f.Client.SendNotice(roomID, usage)
}

// spiteTech sends a fallacy sticker if the body text mentions "firefox".
func (f *Fallacy) spiteTech(body, roomID string) bool {
	l := strings.ToLower(body)
	if strings.Contains(l, "firefox") {
		_, err := f.Client.SendSticker(roomID, "👨 (man)", "mxc://spitetech.com/XFgJMFCXulNthUiFUDqoEzuD")
		if err != nil {
			log.Println("sending sticker failed with error:", err)
		}
		return true
	}
	return false
}

// HandleUserPolicy handles `m.policy.rule.user` events`.
func (f *Fallacy) HandleUserPolicy(ev *gomatrix.Event) {
	r, ok := ev.Content["recommendation"].(string)
	if !ok {
		log.Println("type assert failed when asserting `recommendation` key, not a string")
		return
	}
	switch r {
	case "m.ban":
	case "org.matrix.mjolnir.ban":
		f.BanUserGlobAll(ev.Content["entity"].(string))
	}
}

// HandleMember handles `m.room.member` events
func (f *Fallacy) HandleMember(ev *gomatrix.Event) {
	sender, room := ev.Sender, ev.RoomID
	display, ok := ev.Content["displayname"].(string)
	if !ok {
		display = sender
	}

	if f.Config.Welcome && isNewJoin(ev) {
		f.WelcomeMember(display, sender, room)
	}
}

// HandleMessage handles m.room.message events
func (f *Fallacy) HandleMessage(ev *gomatrix.Event) {
	b, _ := ev.Body() // body is required

	if f.Config.Firefox {
		if d := f.spiteTech(b, ev.RoomID); d {
			return
		}
	}

	if !strings.HasPrefix(b, "!"+f.Config.Name) {
		return
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

// HandleTombStone handles m.room.tombstone events
func (f *Fallacy) HandleTombstone(ev *gomatrix.Event) {
	r := ev.Content["replacement_room"].(string) // `replacement_room` is required by spec

	_, err := f.Client.JoinRoom(r, "", map[string]string{"reason": "following room upgrade"})
	if err != nil {
		log.Println("attempting to join `replacement_room` failed with error:", err)
	}
}
