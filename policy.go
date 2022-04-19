// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"log"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"golang.org/x/sync/errgroup"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var errNotUser error = errors.New("could not ban user, not a valid glob or user id!")

/*
 * // isRetry determines if the error is a 429.
 * func isRetry(err error) bool {
 * 	if e, ok := err.(mautrix.HTTPError); ok {
 * 		if e.IsStatus(429) {
 * 			return true
 * 		}
 * 	}
 * 	return false
 * }
 */

type options struct {
	User   string
	RoomID id.RoomID

	Joined *mautrix.RespJoinedMembers
	Power  *event.PowerLevelsEventContent
}

// globBan bans any member in the room matching the specified glob. It returns
// an error on the first non-nil error.
func globBan(roomID id.RoomID, glb glob.Glob, jm *mautrix.RespJoinedMembers,
	pl *event.PowerLevelsEventContent) (err error) {
	if jm == nil {
		jm, err = Client.JoinedMembers(roomID)
		if err != nil {
			return
		}
	}

	if pl == nil {
		p, err := powerLevels(roomID)
		if err != nil {
			return err
		}
		pl = &p
	}

	var g errgroup.Group
	for user := range jm.Joined {
		if m := minAdmin(pl); !glb.Match(string(user)) || pl.GetUserLevel(user) >= m {
			continue
		}
		u := user
		g.Go(func() error {
			_, err := Client.BanUser(roomID, &mautrix.ReqBanUser{
				Reason: "u jus got glob ban",
				UserID: u,
			})
			return err
		})
	}
	return g.Wait()
}

// Interactively bans a user based on whether they are a glob or a MXID.
//
// The glob matching is able to do matching with a literal but structuring it
// this way allows users to preemptively ban problematic users.
func matchBan(opt options) error {
	switch {
	case strings.Contains(opt.User, "*"), strings.Contains(opt.User, "?"):
		glb, err := glob.Compile(opt.User)
		if err != nil {
			return err
		}
		return globBan(opt.RoomID, glb, opt.Joined, opt.Power)
	case strings.HasPrefix(opt.User, "@"):
		_, err := Client.BanUser(opt.RoomID, &mautrix.ReqBanUser{
			UserID: id.UserID(opt.User),
		})
		return err
	}
	return errNotUser
}

func MatchBan(user string, roomID id.RoomID) error {
	return matchBan(options{
		User:   user,
		RoomID: roomID,
	})
}

func MatchMembers(user string, roomID id.RoomID, jm *mautrix.RespJoinedMembers,
	pl *event.PowerLevelsEventContent) error {
	return matchBan(options{
		User:   user,
		RoomID: roomID,
		Joined: jm,
		Power:  pl,
	})

}

// BanUser bans a glob or MXID from the room.
func BanUser(body []string, ev event.Event) {
	pl, err := powerLevels(ev.RoomID)
	if err != nil {
		sendNotice(ev.RoomID, "fetching power levels event failed with error "+err.Error())
		return
	}

	if pl.Ban() > pl.GetUserLevel(Client.UserID) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	user := body[0]
	if err := MatchBan(user, ev.RoomID); err == errNotUser {
		sendNotice(ev.RoomID, err.Error())
		return
	} else if err != nil {
		sendNotice(ev.RoomID, "failed to ban with: "+err.Error())
		log.Println(err)
		return
	}
	sendNotice(ev.RoomID, "finished banning user!")
}

func banType(s map[event.Type]map[string]*event.Event, t event.Type, roomID id.RoomID) error {
	var once sync.Once

	pl, err := powerLevels(roomID)
	if err != nil {
		return err
	}

	jm, err := Client.JoinedMembers(roomID)
	if err != nil {
		return err
	}

	for k, e := range s[t] {
		r, ok := e.Content.Raw["recommendation"].(string)
		if !ok {
			continue
		}

		switch r {
		case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove legacy mjolnir ban
			_, err := Client.SendStateEvent(roomID, event.StatePolicyUser, k, &e.Content)
			if err != nil {
				log.Println(err)
				once.Do(func() {
					sendNotice(roomID, "could not send state event, proceeding")
				})
			}

			e, ok := e.Content.Raw["entity"].(string)
			if !ok {
				break
			}
			if err := MatchMembers(e, roomID, jm, &pl); err != nil {
				log.Println(err)
			}
		}
	}
	return nil
}

// ImportList imports a banlist from another room.
func ImportList(body []string, ev event.Event) {
	if !hasPerms(ev.RoomID, event.StatePolicyUser) || !hasPerms(ev.RoomID, event.StateMember) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	roomID := id.RoomID(body[0])
	if p := roomID[0]; p == '#' {
		r, err := Client.ResolveAlias(id.RoomAlias(roomID))
		if err != nil {
			sendNotice(ev.RoomID, "Could not resolve room alias, failed with "+err.Error())
			return
		}
		roomID = r.RoomID
	} else if p != '!' {
		sendNotice(ev.RoomID, "Not a valid room ID!")
		return
	}

	if roomID == ev.RoomID {
		sendNotice(ev.RoomID, "Refusing to import events from this room!")
		return
	}

	if _, err := Client.JoinRoomByID(roomID); err != nil {
		msg := strings.Join([]string{"could not join room", body[0], "failed with:", err.Error()}, " ")
		sendNotice(ev.RoomID, msg)
		return
	}

	s, err := Client.State(roomID)
	if err != nil {
		msg := strings.Join([]string{"could not import state from", body[0], "failed with:", err.Error()}, " ")
		sendNotice(ev.RoomID, msg)
	}

	banType(s, event.StatePolicyUser, ev.RoomID)
	banType(s, event.NewEventType("m.room.rule.user"), ev.RoomID)
	sendNotice(ev.RoomID, "Finished importing list from "+body[0])
}
