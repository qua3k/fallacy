// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"errors"
	"strings"

	"github.com/gobwas/glob"
	"golang.org/x/sync/errgroup"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// the options struct for banning people
type options[T modReq, U modResp] struct {
	userID string

	// the glob compiled from userID
	glb    glob.Glob
	roomID id.RoomID

	members *mautrix.RespJoinedMembers
	power   *event.PowerLevelsEventContent

	// action to take if a joined member matches the userID
	action func(id.RoomID, *T) (*U, error)
}

// init ensures that options struct has power levels and joined_members,
// otherwise error out
func (o *options[T, U]) init() error {
	if o.members == nil {
		m, err := Client.JoinedMembers(o.roomID)
		if err != nil {
			return err
		}
		o.members = m
	}
	if o.power == nil {
		p, err := powerLevels(o.roomID)
		if err != nil {
			return err
		}
		o.power = p
	}
	return nil
}

type (
	modReq interface {
		mautrix.ReqKickUser | mautrix.ReqBanUser
	}
	modResp interface {
		mautrix.RespKickUser | mautrix.RespBanUser
	}
)

// globMatch is a generic function to kick or ban joined users matching the glob
// from the room. It returns an error on the first non-nil error.
func (o options[T, U]) globMatch() error {
	if err := o.init(); err != nil {
		return err
	}
	lvl := adminLevel(o.power)

	var g errgroup.Group
	for user := range o.members.Joined {
		u := user
		if !o.glb.Match(string(u)) || o.power.GetUserLevel(u) >= lvl {
			continue
		}
		g.Go(func() error {
			_, err := o.action(o.roomID, &T{
				Reason: "u just got globbed",
				UserID: u,
			})
			return err
		})
	}
	return g.Wait()
}

// Interactively action on a user based on whether they are a glob or a MXID.
//
// The glob matching is able to do matching with a literal but structuring it
// this way allows users to preemptively ban problematic users.
func (o options[T, U]) dispatchAction() error {
	switch {
	case strings.Contains(o.userID, "*"), strings.Contains(o.userID, "?"):
		glb, err := glob.Compile(o.userID)
		if err != nil {
			return err
		}
		o.glb = glb
		return o.globMatch()
	case o.userID[0] == '@':
		_, err := o.action(o.roomID, &T{UserID: id.UserID(o.userID)})
		return err
	}
	return errNotUser
}

func moderateUser[T modReq, U modResp](roomID id.RoomID, userID string,
	f func(id.RoomID, *T) (*U, error)) error {
	pl, err := powerLevels(roomID)
	if err != nil {
		return err
	}

	if pl.Ban() > pl.GetUserLevel(Client.UserID) {
		return errNoPerms
	}

	jm, err := Client.JoinedMembers(roomID)
	if err != nil {
		return err
	}

	opt := options[T, U]{
		userID:  userID,
		roomID:  roomID,
		power:   pl,
		members: jm,
		action:  f,
	}
	return opt.dispatchAction()
}

func BanUser(body []string, ev event.Event) {
	if err := moderateUser(ev.RoomID, body[0], Client.BanUser); err != nil {
		sendNotice(ev.RoomID, "banning user failed with", err.Error())
	}
}

func KickUser(body []string, ev event.Event) {
	if err := moderateUser(ev.RoomID, body[0], Client.KickUser); err != nil {
		sendNotice(ev.RoomID, "kicking user failed with", err.Error())
	}
}

func (o options[T, U]) processBans(evs map[string]*event.Event) error {
	for key, ev := range evs {
		r, ok := ev.Content.Raw["recommendation"].(string)
		if !ok {
			continue
		}

		e, ok := ev.Content.Raw["entity"].(string)
		if !ok {
			continue
		}
		o.userID = e

		switch r {
		case "m.ban", "org.matrix.mjolnir.ban": // TODO: remove legacy mjolnir ban
			_, err := Client.SendStateEvent(o.roomID, event.StatePolicyUser, key, &ev.Content)
			if err != nil {
				return err
			}
			if o.dispatchAction() != nil {
				return err
			}
		}
	}
	return nil
}

func resolveRoom(roomID string) (id.RoomID, error) {
	switch roomID[0] {
	case '#':
		r, err := Client.ResolveAlias(id.RoomAlias(roomID))
		if err != nil {
			return "", err
		}
		return r.RoomID, nil
	case '!':
		return id.RoomID(roomID), nil
	}
	return "", errInvalidRoom
}

// ImportList imports a banlist from another room.
func ImportList(body []string, ev event.Event) {
	if !hasPerms(ev.RoomID, event.StatePolicyUser) || !hasPerms(ev.RoomID, event.StateMember) {
		sendNotice(ev.RoomID, permsMessage)
		return
	}

	powerLevels(ev.RoomID)

	roomID, err := resolveRoom(body[0])
	if err != nil {
		sendNotice(ev.RoomID, err.Error())
		return
	}

	if roomID == ev.RoomID {
		sendNotice(ev.RoomID, "Refusing to import events from this room!")
		return
	}

	_, hs, _ := ev.Sender.ParseAndDecode()
	if _, err := Client.JoinRoom(string(roomID), hs, nil); err != nil {
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

	opt := options[mautrix.ReqBanUser, mautrix.RespBanUser]{
		roomID:  ev.RoomID,
		members: jm,
		power:   pl,
		action:  Client.BanUser,
	}
	if err = opt.processBans(s[event.StatePolicyUser]); err != nil {
		sendNotice(ev.RoomID, "processing bans failed with", err.Error())
		return
	}
	opt.processBans(s[event.NewEventType("m.room.rule.user")])
	sendNotice(ev.RoomID, "Finished importing list from", body[0])
}

func createBanList(sender id.UserID, room string) (id.RoomID, error) {
	display := string(sender)
	r, err := Client.GetDisplayName(sender)
	if err == nil {
		display = r.DisplayName
	}

	resp, err := Client.CreateRoom(&mautrix.ReqCreateRoom{
		Invite: []id.UserID{sender},
		Name:   room,
		PowerLevelOverride: &event.PowerLevelsEventContent{
			EventsDefault: 50,
			Users: map[id.UserID]int{
				sender:        100,
				Client.UserID: 50,
			},
		},
		Preset: "trusted_private_chat",
		Topic:  "ban list created by " + display,
	})
	return resp.RoomID, err
}

var (
	errInvalidRoom = errors.New("not a valid room ID")
	errMembers     = errors.New("could not fetch joined members")
	errNotUser     = errors.New("could not ban user, not a valid glob or user id")
	errPowerLevels = errors.New("could not fetch power levels")
)
