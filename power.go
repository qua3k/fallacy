// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"

	"github.com/gobwas/glob"
	"github.com/qua3k/gomatrix"
)

// BanUserGlob bans users with the power of glob.
func (f *Fallacy) BanUserGlob(roomID, userGlob string) (err error) {
	g, err := glob.Compile(userGlob)
	if err != nil {
		return
	}

	jm, err := f.Client.JoinedMembers(roomID)
	if err != nil {
		return
	}
	for m := range jm.Joined {
		go func(m string) { // does this race?
			if g.Match(m) {
				_, err := f.Client.BanUser(m, &gomatrix.ReqBanUser{
					Reason: "u jus got glob ban",
					UserID: m,
				})
				if err != nil {
					log.Println("glob banning user failed with:", err)
				}
			}
		}(m)
	}
	return
}

// BanUserGlobAll bans user with the power of glob from all rooms the client is
// joined to.
func (f *Fallacy) BanUserGlobAll(userGlob string) (err error) {
	jr, err := f.Client.JoinedRooms()
	if err != nil {
		return err
	}

	for _, r := range jr.JoinedRooms {
		go func(r string) { // does this race?
			if err := f.BanUserGlob(r, userGlob); err != nil {
				log.Println("attempting to glob ban users failed with:", err)
			}
		}(r)
	}
	return
}
