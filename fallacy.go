// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package fallacy implements the fallacy bot library.
package fallacy

import (
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type Config struct {
	Homeserver string
	Username   string
	Password   string
	Name       string
}

// New initializes the library and should be called before any other functions.
// It is safe to call more than once, as it is only initialized once.
func New(c Config) (err error) {
	once.Do(func() {
		lock.Lock()
		defer lock.Unlock()

		newClient, e := mautrix.NewClient(c.Homeserver, id.UserID(c.Username), "")
		if e != nil {
			err = e
			return
		}

		Client = newClient
		handles = defaultHandles
	})
	return
}

func Login(c Config) error {
	_, err := Client.Login(&mautrix.ReqLogin{
		Identifier: mautrix.UserIdentifier{
			User: c.Username,
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

var (
	// mutex protecting this block
	lock sync.RWMutex

	// initialize this block one time only
	once sync.Once

	// Client is the currently connected Client
	Client *mautrix.Client

	// handles are the current handlers
	handles map[string][]Callback

	// limit are the allowed requests/second
	limit = time.Tick(time.Millisecond * 200)

	// some room specific settings, should be migrated...
	firefox, welcome bool
	// this is supposed to be join rules
	rules []string
)

var defaultHandles = map[string][]Callback{
	"ban":       {{BanUser, 1}},
	"import":    {{ImportList, 1}},
	"mute":      {{MuteUser, 1}},
	"pin":       {{Function: PinMessage}},
	"purge":     {{Function: CommandPurge}},
	"say":       {{SayMessage, 1}},
	"umute":     {{UnmuteUser, 1}},
}
