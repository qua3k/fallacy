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

const usage = `this is how you use the bot
to get rid of your enemies: do 'ban [MEMBER] [REASON]'
to avoid hearing from your friends: do 'mute [MEMBER]'
to get rid of your shitposts: do 'purge [NUMBER]'
to welcome new members: do 'welcome [BOOL]'`

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

// NewConfig: instantiates a new Config struct
func NewConfig(firefox bool, name string, welcome bool) Config {
	return Config{
		Firefox: firefox,
		Name:    name,
		Welcome: welcome,
	}
}

// NewFallacy: instantiates a new Fallacy struct
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

// printHelp: send the help message into a room
func (f *Fallacy) printHelp(roomID string) {
	f.Client.SendNotice(roomID, usage)
}

func (f *Fallacy) HandleMember(ev *gomatrix.Event) {
	s, r := ev.Sender, ev.RoomID
	d, ok := ev.Content["displayname"].(string)
	if !ok {
		d = ev.Sender
	}

	if f.Config.Welcome && !isDisplayOrAvatar(ev) {
		f.WelcomeMember(d, s, r)
	}
}

// HandleMessage: handle m.room.message events
func (f *Fallacy) HandleMessage(ev *gomatrix.Event) {
	b, _ := ev.Body() // body is required
	if !strings.HasPrefix(b, "!"+f.Config.Name) {
		return
	}

	if f.Config.Firefox {
		f.spiteTech(b, ev.RoomID)
		return
	}

	s := strings.Fields(b)
	if len(s) < 3 {
		f.printHelp(ev.RoomID)
	}

	switch s[1] {
	case "mute":
		f.MuteUser(ev.RoomID, ev.Sender, s[2])
	case "purge":
		l, err := strconv.Atoi(s[2])
		if err != nil {
			f.printHelp(ev.RoomID)
			return
		}
		f.PurgeMessages(ev.RoomID, "", l)
	case "unmute":
		f.UnmuteUser(ev.RoomID, ev.Sender, s[2])
	}

	// messages := make(chan string)
}

// HandleTombStone: handle m.room.tombstone events
func (f *Fallacy) HandleTombstone(ev *gomatrix.Event) {
	r := ev.Content["replacement_room"].(string) // `replacement_room` is required by spec
	if _, err := f.Client.JoinRoom(r, "", map[string]string{"reason": "following room upgrade"}); err != nil {
		log.Println("attempting to join `replacement_room` failed with error: ", err)
	}
}
