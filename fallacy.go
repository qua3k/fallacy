// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package fallacy implements the fallacy bot library.
package fallacy

import (
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type Config struct {
	// the homeserver to connect to, e.g., https://matrix-client.matrix.org
	Homeserver string
	// the username (mxid) to connect with, e.g., @fallacy:matrix.org
	Username id.UserID
	// the password to the account
	Password string
	// the name of the bot
	Name string
	// the rooms the bot responds in, omit to allow all rooms
	PermittedRooms []id.RoomID `toml:"permitted_rooms"`
}

// New initializes the library and should be called before any other functions.
// It is safe to call more than once, as it is only initialized once.
func (c Config) New() error {
	lock.Lock()
	defer lock.Unlock()

	if !once {
		client, err := mautrix.NewClient(c.Homeserver, c.Username, "")
		if err != nil {
			return err
		}

		Client = client
		once = true
		permittedRooms = c.PermittedRooms
	}
	return nil
}

func (c Config) Login() error {
	_, err := Client.Login(&mautrix.ReqLogin{
		Identifier: mautrix.UserIdentifier{
			User: string(c.Username),
			Type: mautrix.IdentifierTypeUser,
		},
		InitialDeviceDisplayName: c.Name,
		Password:                 c.Password,
		StoreCredentials:         true,
		Type:                     mautrix.AuthTypePassword,
	})
	return err
}

func enableFirefox(b bool) {
	lock.Lock()
	defer lock.Unlock()
	firefox = b
}

func enableWelcome(b bool) {
	lock.Lock()
	defer lock.Unlock()
	welcome = b
}

var defaultHandles = map[string][]Callback{
	"ban":    {{BanUser, 1}},
	"import": {{ImportList, 1}},
	"mute":   {{MuteUser, 1}},
	"pin":    {{Function: PinMessage}},
	"purge":  {{Function: CommandPurge}},
	"say":    {{SayMessage, 1}},
	"umute":  {{UnmuteUser, 1}},
}

var (
	// mutex protecting this block
	lock sync.RWMutex

	// initialize this block one time only
	once bool

	// Client is the currently connected Client
	Client *mautrix.Client

	// handles are the current handlers
	handles = defaultHandles

	// limit are the allowed requests/second
	limit = time.NewTicker(time.Millisecond * 200).C

	pool pgxpool.Pool

	// some room specific settings, should be migrated...
	firefox, welcome bool
	// this is supposed to be join rules
	rules []string

	permittedRooms []id.RoomID
)
