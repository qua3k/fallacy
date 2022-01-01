// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"
	"sync"

	"github.com/gobwas/glob"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// GlobBan bans a single user from the room with the power of glob.
func (f *Fallacy) GlobBan(glob glob.Glob, roomID id.RoomID, userID id.UserID) (err error) {
	if glob.Match(string(userID)) {
		_, err = f.Client.BanUser(roomID, &mautrix.ReqBanUser{
			Reason: "u jus got glob ban",
			UserID: userID,
		})
	}
	return
}

// GlobBanRoom utilizes the power of glob to ban all users matching the glob
// from the room.
func (f *Fallacy) GlobBanRoom(glob glob.Glob, roomID id.RoomID) (err error) {
	jm, err := f.Client.JoinedMembers(roomID)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for u := range jm.Joined {
		wg.Add(1)
		go func(u id.UserID) {
			defer wg.Done()
			if err := f.GlobBan(glob, roomID, u); err != nil {
				log.Println(err)
			}
		}(u)
	}
	wg.Wait()
	return
}

// GlobBanAll uses glob to ban users from all rooms the client is joined to.
func (f *Fallacy) GlobBanAll(userGlob string) (err error) {
	jr, err := f.Client.JoinedRooms()
	if err != nil {
		return
	}

	g, err := glob.Compile(userGlob)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for _, r := range jr.JoinedRooms {
		wg.Add(1)
		go func(r id.RoomID) {
			if err := f.GlobBanRoom(g, r); err != nil {
				log.Printf("attempting to glob ban users from room %s failed with: %v", r, err)
			}
		}(r)
	}
	wg.Wait()
	return
}

// AddJoinRule adds a join rule to ban users on sight.
func (f *Fallacy) AddJoinRule(rule string) {
	f.Config.Lock.RLock()
	for _, r := range f.Config.Rules {
		if r == rule {
			return
		}
	}
	f.Config.Lock.RUnlock()

	f.Config.Lock.Lock()
	defer f.Config.Lock.Unlock()
	f.Config.Rules = append(f.Config.Rules, rule)
}

// DeleteJoinRule deletes the rule, if it exists.
func (f *Fallacy) DeleteJoinRule(rule string) {
	f.Config.Lock.Lock()
	defer f.Config.Lock.Unlock()
	for i, r := range f.Config.Rules {
		if r == rule {
			f.Config.Rules = append(f.Config.Rules[:i], f.Config.Rules[i+1:]...)
		}
	}
}
