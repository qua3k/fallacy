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

// attemptSendNotice wraps Client.SendNotice, logging when a notice is unable to
// be sent.
func (f *Fallacy) attemptSendNotice(roomID id.RoomID, text string) {
	if _, err := f.Client.SendNotice(roomID, text); err == nil {
		return
	}
	log.Printf("could not send notice '%s' into room %s!\n", text, roomID.String())
}

// AddJoinRule adds a join rule to ban users on sight.
func (f *Fallacy) AddJoinRule(rule string) {
	f.Config.Lock.Lock() // TODO: evaluate speed of rlock/runlock/lock/unlock
	defer f.Config.Lock.Unlock()

	for _, r := range f.Config.Rules {
		if r == rule {
			return
		}
	}
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

// GlobBanUser bans a single user from the room if it matches the supplied glob,
// returning an error if unsuccessful.
func (f *Fallacy) GlobBanUser(glob glob.Glob, roomID id.RoomID, userID id.UserID) (err error) {
	if userString := userID.String(); !glob.Match(userString) {
		return
	}
	_, err = f.Client.BanUser(roomID, &mautrix.ReqBanUser{
		Reason: "u jus got glob ban",
		UserID: userID,
	})
	return
}

// GlobBanJoinedMembers utilizes the power of glob to ban all users matching
// the glob from the room, returning an error if unsuccessful. This does not
// attempt to ban admins, sending a notice when an attempt is made to ban an
// admin or banning a user fails.
func (f *Fallacy) GlobBanJoinedMembers(glob glob.Glob, roomID id.RoomID) (err error) {
	jm, err := f.Client.JoinedMembers(roomID)
	if err != nil {
		return
	}

	pl, err := f.powerLevels(roomID)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for user := range jm.Joined {
		wg.Add(1)
		go func(u id.UserID) {
			defer wg.Done()
			if isAdmin(pl, roomID, u) {
				const adminBanMessage = "cannot ban an admin, try demoting them first..."
				f.attemptSendNotice(roomID, adminBanMessage)
				return
			}
			if err := f.GlobBanUser(glob, roomID, user); err == nil {
				return
			}
			f.attemptSendNotice(roomID, err.Error())
		}(user)
	}

	wg.Wait()
	return
}

// GlobBanJoinedRooms utilizes the power of glob to ban users matching the glob
// from all rooms the client is joined to, returning an error if unsuccessful.
func (f *Fallacy) GlobBanJoinedRooms(glob glob.Glob) (err error) {
	jr, err := f.Client.JoinedRooms()
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for _, room := range jr.JoinedRooms {
		wg.Add(1)
		go func(r id.RoomID) {
			defer wg.Done()
			if err := f.GlobBanJoinedMembers(glob, r); err != nil {
				log.Println(err)
			}
		}(room)
	}

	wg.Wait()
	return
}
