// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
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

const usage = `
(  __)/ _\ (  )  (  )   / _\  / __)( \/ )
) _)/    \/ (_/\/ (_/\/    \( (__  )  / 
(__) \_/\_/\____/\____/\_/\_/ \___)(__/  

Usage: fallacy <config file>`

func init() {
	rand.Seed(time.Now().UnixNano())
}
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	var c fallacy.Config
	if _, err := toml.DecodeFile(os.Args[1], &c); err != nil {
		fmt.Fprintln(os.Stderr, "decoding config file failed with", err)
		os.Exit(1)
	}

	err := c.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "creating fallacy struct failed with", err)
		os.Exit(1)
	}

	err = c.Login()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging into %s failed with %v\n", c.Homeserver, err)
		os.Exit(1)
	}

	syncer := fallacy.NewFallacySyncer()
	syncer.OnEventType(event.StatePolicyUser, fallacy.HandleUserPolicy)
	syncer.OnEventType(event.StateMember, fallacy.HandleMember)
	syncer.OnEventType(event.EventMessage, fallacy.HandleMessage)
	syncer.OnEventType(event.StateTombstone, fallacy.HandleTombstone)

	old := &mautrix.OldEventIgnorer{
		UserID: fallacy.Client.UserID,
	}
	old.Register(syncer)
	fallacy.Client.Syncer = syncer

	if err := fallacy.Client.Sync(); err != nil {
		log.Println("Sync() returned", err)
	}
}
