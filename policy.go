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

// the options struct for banning people
type options struct {
	User   string
	RoomID id.RoomID

	Members *mautrix.RespJoinedMembers
	Power   *event.PowerLevelsEventContent
}

// globBan bans any member in the room matching the specified glob. It returns
// an error on the first non-nil error.
func globBan(roomID id.RoomID, glb glob.Glob) (err error) {
	pl, err := powerLevels(roomID)
	if err != nil {
		return
	}

	jm, err := Client.JoinedMembers(roomID)
	if err != nil {
		return
	}

	var g errgroup.Group
	for user := range jm.Joined {
		m, u := minAdmin(pl), user
		if !glb.Match(string(user)) || pl.GetUserLevel(user) >= m {
			continue
		}

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

// globBanState is like glob ban but takes power levels and a joined_members
// response. Useful for banning lots of people.
func globBanState(roomID id.RoomID, glb glob.Glob, members *mautrix.RespJoinedMembers,
	power *event.PowerLevelsEventContent) (err error) {

	var g errgroup.Group
	for user := range members.Joined {
		m, u := minAdmin(power), user
		if !glb.Match(string(user)) || power.GetUserLevel(user) >= m {
			continue
		}

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

// BanUser bans a glob or MXID from the room.
func BanUser(body []string, ev event.Event) {
	pl, err := powerLevels(ev.RoomID)
	if err != nil {
		sendNotice(ev.RoomID, errPowerLevels.Error(), "failed with error", err.Error())
		return
	}

	if pl.Ban() > pl.GetUserLevel(Client.UserID) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	jm, err := Client.JoinedMembers(ev.RoomID)
	if err != nil {
		sendNotice(ev.RoomID, errMembers.Error(), "failed with", err.Error())
	}
	matchBan(options{body[0], ev.RoomID, jm, pl})
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

		if opt.Power == nil {
			return globBan(opt.RoomID, glb)
		}
		return globBanState(opt.RoomID, glb, opt.Members, opt.Power)
	case opt.User[0] == '@':
		_, err := Client.BanUser(opt.RoomID, &mautrix.ReqBanUser{
			UserID: id.UserID(opt.User),
		})
		return err
	}
	return errNotUser
}

func processBans(evs map[string]*event.Event, roomID id.RoomID, members *mautrix.RespJoinedMembers,
	power *event.PowerLevelsEventContent) {
	var once sync.Once
	for key, ev := range evs {
		r, ok := ev.Content.Raw["recommendation"].(string)
		if !ok {
			continue
		}

		e, ok := ev.Content.Raw["entity"].(string)
		if !ok {
			continue
		}

		switch r {
		case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove legacy mjolnir ban
			_, err := Client.SendStateEvent(roomID, event.StatePolicyUser, key, &ev.Content)
			if err != nil {
				once.Do(func() {
					sendNotice(roomID, "could not send state event, proceeding")
				})
				log.Println(err)
			}
			matchBan(options{e, roomID, members, power})
		}
	}
}

// ImportList imports a banlist from another room.
func ImportList(body []string, ev event.Event) {
	if !hasPerms(ev.RoomID, event.StatePolicyUser) || !hasPerms(ev.RoomID, event.StateMember) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	roomID := id.RoomID(body[0])
	if roomID[0] == '#' {
		r, err := Client.ResolveAlias(id.RoomAlias(roomID))
		if err != nil {
			sendNotice(ev.RoomID, "Could not resolve room alias, failed with", err.Error())
			return
		}
		roomID = r.RoomID
	} else if roomID[0] != '!' {
		sendNotice(ev.RoomID, "Not a valid room ID!")
		return
	}

	if roomID == ev.RoomID {
		sendNotice(ev.RoomID, "Refusing to import events from this room!")
		return
	}

	if _, err := Client.JoinRoomByID(roomID); err != nil {
		sendNotice(ev.RoomID, "could not join room", roomID.String(), "failed with:", err.Error())
		return
	}

	s, err := Client.State(roomID)
	if err != nil {
		sendNotice(ev.RoomID, "could not import state from", roomID.String(), "failed with:", err.Error())
		return
	}

	pl, err := powerLevels(ev.RoomID)
	if err != nil {
		sendNotice(ev.RoomID, errPowerLevels.Error(), "failed with error", err.Error())
		return
	}

	jm, err := Client.JoinedMembers(ev.RoomID)
	if err != nil {
		sendNotice(ev.RoomID, errMembers.Error(), "failed with", err.Error())
		return
	}

	processBans(s[event.StatePolicyUser], ev.RoomID, jm, pl)
	processBans(s[event.NewEventType("m.room.rule.user")], ev.RoomID, jm, pl)
	sendNotice(ev.RoomID, "Finished importing list from", body[0])
}

var (
	errNotUser     error = errors.New("could not ban user, not a valid glob or user id!")
	errPowerLevels error = errors.New("could not fetch power levels!")
	errMembers     error = errors.New("could not fetch joined members!")
)
