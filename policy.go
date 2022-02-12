// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"
	"strings"

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

type ban struct {
	Glob   glob.Glob
	RoomID id.RoomID

	Join  mautrix.RespJoinedMembers
	Power event.PowerLevelsEventContent
}

func (f *Fallacy) banUsers(b ban) {
	for user := range b.Join.Joined {
		if u := user.String(); !b.Glob.Match(u) {
			continue
		}
		if isAdmin(b.Power, b.RoomID, user) {
			const adminBanMessage = "Haha, let's /demote him first."
			f.attemptSendNotice(b.RoomID, adminBanMessage)
			return
		}
		if err := f.banWithReason(b.RoomID, user, globBanReason); err != nil {
			f.attemptSendNotice(b.RoomID, err.Error())
		}
	}
}

// BanUsers glob bans a slice of users expressed as globs.
func (f *Fallacy) BanUsers(globs []string, ev event.Event) {
	pl, err := f.powerLevels(ev.RoomID)
	if err != nil {
		log.Println(err)
		return
	}

	if pl.Ban() > pl.GetUserLevel(f.Client.UserID) {
		f.attemptSendNotice(ev.RoomID, noPermsMessage)
		return
	}

	jm, err := f.Client.JoinedMembers(ev.RoomID)
	if err != nil {
		return
	}

	for _, glb := range globs {
		go func(glb string) {
			g, err := glob.Compile(glb)
			if err != nil {
				msg := strings.Join([]string{"compiling glob", glb, "failed!"}, " ")
				f.attemptSendNotice(ev.RoomID, msg)
				return
			}
			f.banUsers(ban{
				Glob:   g,
				RoomID: ev.RoomID,
				Join:   *jm,
				Power:  pl,
			})
		}(glb)
	}
}

// GlobBanJoinedRooms utilizes the power of glob to ban users matching the glob
// from all rooms the client is joined to, returning an error if unsuccessful.
func (f *Fallacy) GlobBanJoinedRooms(glob glob.Glob) (err error) {
	jr, err := f.Client.JoinedRooms()
	if err != nil {
		return
	}

	for _, room := range jr.JoinedRooms {
		go func(r id.RoomID) {
			jm, err := f.Client.JoinedMembers(r)
			if err != nil {
				log.Println(err)
				return
			}
			pl, err := f.powerLevels(r)
			if err != nil {
				log.Println(err)
				return
			}
			f.banUsers(ban{
				Glob:   glob,
				RoomID: r,
				Join:   *jm,
				Power:  pl,
			})
		}(room)
	}
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

	for _, room := range jr.JoinedRooms {
		go func(r id.RoomID) {
			if err := f.BanServer(r, homeserverID); err != nil {
				msg := strings.Join([]string{"unable to ban", r.String(), "from room,", "failed with error:", err.Error()}, " ")
				log.Println(msg)
			}
		}(room)
	}
	return
}
