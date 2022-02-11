// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/qua3k/fallacy"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

const usage = `Usage:
	fallacy -c config file`

type tomlStruct struct {
	Config        fallacy.Config
	HomeserverUrl string
	Username      string
	Password      string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {

	var (
		configFile string = "config.toml"
		homeserver string = "https://matrix-client.matrix.org"
		t          tomlStruct
	)

	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flag.StringVar(&configFile, "c", "", "config from `FILE` (default config.toml)")
	flag.StringVar(&configFile, "config", "", "config from `FILE` (default config.toml)")
	flag.Parse()

	if len(os.Args) < 3 {
		flag.Usage()
		os.Exit(1)
	}

	md, err := toml.DecodeFile(configFile, &t)
	if err != nil {
		log.Fatal("decoding config file failed with error:", err)
	}

	if md.IsDefined("homeserverUrl") {
		homeserver = t.HomeserverUrl
	}

	f, err := fallacy.NewFallacy(homeserver, "", "", &t.Config)
	if err != nil {
		log.Fatal("creating fallacy struct failed with:", err)
	}

	_, err = f.Client.Login(&mautrix.ReqLogin{
		Identifier: mautrix.UserIdentifier{
			User: t.Username,
			Type: mautrix.IdentifierTypeUser,
		},
		InitialDeviceDisplayName: t.Config.Name,
		Password:                 t.Password,
		StoreCredentials:         true,
		Type:                     "m.login.password",
	})
	if err != nil {
		log.Fatalln("login failed with:", err)
	}

	syncer := fallacy.NewFallacySyncer()
	f.Client.Syncer = syncer
	syncer.OnEventType(event.StatePolicyUser, f.HandleUserPolicy)
	syncer.OnEventType(event.StateMember, f.HandleMember)
	syncer.OnEventType(event.EventMessage, f.HandleMessage)
	syncer.OnEventType(event.StateTombstone, f.HandleTombstone)
	f.Register("ban", fallacy.Callback{
		Function: f.BanUsers,
		MinArgs:  1,
	})
	f.Register("mute", fallacy.Callback{
		Function: f.MuteUsers,
		MinArgs:  1,
	})
	f.Register("pin", fallacy.Callback{
		Function: f.PinMessage,
	})
	f.Register("purge", fallacy.Callback{
		Function: f.PurgeMessages,
	})
	f.Register("purgeuser", fallacy.Callback{
		Function: f.PurgeUsers,
		MinArgs:  1,
	})
	f.Register("say", fallacy.Callback{
		Function: f.SayMessage,
		MinArgs:  1,
	})
	f.Register("unmute", fallacy.Callback{
		Function: f.MuteUsers,
		MinArgs:  1,
	})

	old := &mautrix.OldEventIgnorer{
		UserID: f.Client.UserID,
	}

	old.Register(syncer)

	if err := f.Client.Sync(); err != nil {
		fmt.Println("Sync() returned", err)
	}
}
