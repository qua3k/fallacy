// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// the reason for the season
const globBanReason = "u jus got glob ban"

// attemptSendNotice wraps Client.SendNotice, logging when a notice is unable to
// be sent.
func (f *Fallacy) attemptSendNotice(roomID id.RoomID, text string) {
	if _, err := f.Client.SendNotice(roomID, text); err == nil {
		return
	}
	msg := strings.Join([]string{"could not send notice", text, "into room", roomID.String()}, " ")
	log.Println(msg)
}

// AddJoinRule adds a join rule to ban users on sight.
func (f *Fallacy) AddJoinRule(rule string) {
	// TODO: evaluate speed of rlock/runlock/lock/unlock
	f.Config.Lock.Lock()
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
		if r != rule {
			continue
		}
		f.Config.Rules = append(f.Config.Rules[:i], f.Config.Rules[i+1:]...)
	}
}

func (f *Fallacy) banWithReason(roomID id.RoomID, userID id.UserID, reason string) (err error) {
	_, err = f.Client.BanUser(roomID, &mautrix.ReqBanUser{
		Reason: reason,
		UserID: userID,
	})
	return
}

// GlobBanSlice glob bans a slice of globs.
func (f *Fallacy) GlobBanSlice(globs []string, ev event.Event) {
	roomID, userID := parseMessage(ev)
	if !f.isAdmin(roomID, userID) {
		f.attemptSendNotice(roomID, "shut up ur not admin")
		return
	}
	for _, u := range globs {
		glob, err := glob.Compile(u)
		if err != nil {
			msg := "compiling glob " + u + " failed!"
			f.attemptSendNotice(roomID, msg)
			return
		}
		f.GlobBanJoinedMembers(glob, roomID)
	}
}

// GlobBanUser bans a single user from the room if it matches the supplied glob,
// returning an error if unsuccessful.
func (f *Fallacy) GlobBanUser(glob glob.Glob, roomID id.RoomID, userID id.UserID) (err error) {
	if userString := userID.String(); !glob.Match(userString) {
		return
	}
	err = f.banWithReason(roomID, userID, globBanReason)
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
			if uString := u.String(); !glob.Match(uString) {
				return
			}
			if isAdmin(&pl, roomID, u) {
				const adminBanMessage = "Haha, let's /demote him first."
				f.attemptSendNotice(roomID, adminBanMessage)
				return
			}
			if err := f.banWithReason(roomID, u, globBanReason); err != nil {
				f.attemptSendNotice(roomID, err.Error())
			}
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

// BanServerJoinedRooms utilizes the power of ACL to add the server to ban
// servers matching the ACL from all rooms the client is joined to, returning an
// error if unsuccessful.
func (f *Fallacy) BanServerJoinedRooms(homeserverID string) (err error) {
	jr, err := f.Client.JoinedRooms()
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for _, room := range jr.JoinedRooms {
		wg.Add(1)
		go func(r id.RoomID) {
			defer wg.Done()
			if err := f.BanServer(r, homeserverID); err != nil {
				msg := strings.Join([]string{"unable to ban", r.String(), "from room,", "failed with error:", err.Error()}, " ")
				log.Println(msg)
			}
		}(room)
	}
	wg.Wait()
	return
}
