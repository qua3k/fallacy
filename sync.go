// Copyright (c) 2020 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fallacy

import (
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type Syncer struct {
	globalListeners []mautrix.EventHandler
	// listeners want a specific event type
	listeners map[event.Type][]mautrix.EventHandler
	// ParseEventContent determines whether or not event content should be parsed before passing to handlers.
	ParseEventContent bool
	// ParseErrorHandler is called when event.Content.ParseRaw returns an error.
	// If it returns false, the event will not be forwarded to listeners.
	ParseErrorHandler func(ev *event.Event, err error) bool
	syncListeners     []mautrix.SyncHandler
}

func NewSyncer() *Syncer {
	return &Syncer{
		listeners:         make(map[event.Type][]mautrix.EventHandler),
		ParseEventContent: true,
		ParseErrorHandler: func(evt *event.Event, err error) bool {
			return false
		},
	}
}

// ProcessResponse processes the /sync response in a way suitable for bots. "Suitable for bots" means a stream of
// unrepeating events. Returns a fatal error if a listener panics.
func (s *Syncer) ProcessResponse(res *mautrix.RespSync, since string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w since=%s panic=%s\n%s", errProcessPanic, since, r, debug.Stack())
		}
	}()

	for _, listener := range s.syncListeners {
		if !listener(res, since) {
			return
		}
	}

	for roomID, roomData := range res.Rooms.Join {
		s.processSyncEvents(roomID, roomData.State.Events, mautrix.EventSourceJoin|mautrix.EventSourceState)
		s.processSyncEvents(roomID, roomData.Timeline.Events, mautrix.EventSourceJoin|mautrix.EventSourceTimeline)
	}
	return
}

func (s *Syncer) processSyncEvents(roomID id.RoomID, events []*event.Event, source mautrix.EventSource) {
	for _, evt := range events {
		s.processSyncEvent(roomID, evt, source)
	}
}

func (s *Syncer) processSyncEvent(roomID id.RoomID, evt *event.Event, source mautrix.EventSource) {
	evt.RoomID = roomID

	// Ensure the type class is correct. It's safe to mutate the class since the event type is not a pointer.
	// Listeners are keyed by type structs, which means only the correct class will pass.
	switch {
	case evt.StateKey != nil:
		evt.Type.Class = event.StateEventType
	case source == mautrix.EventSourcePresence, source&mautrix.EventSourceEphemeral != 0:
		evt.Type.Class = event.EphemeralEventType
	case source&mautrix.EventSourceAccountData != 0:
		evt.Type.Class = event.AccountDataEventType
	case source == mautrix.EventSourceToDevice:
		evt.Type.Class = event.ToDeviceEventType
	default:
		evt.Type.Class = event.MessageEventType
	}

	if s.ParseEventContent {
		err := evt.Content.ParseRaw(evt.Type)
		if err != nil && !s.ParseErrorHandler(evt, err) {
			return
		}
	}

	s.notifyListeners(source, evt)
}

func (s *Syncer) notifyListeners(source mautrix.EventSource, evt *event.Event) {
	listeners, exists := s.listeners[evt.Type]
	if exists {
		for _, fn := range listeners {
			go fn(source, evt)
		}
	}
}

// OnEventType allows callers to be notified when there are new events for the given event type.
// There are no duplicate checks.
func (s *Syncer) OnEventType(eventType event.Type, callback mautrix.EventHandler) {
	_, exists := s.listeners[eventType]
	if !exists {
		s.listeners[eventType] = []mautrix.EventHandler{}
	}
	s.listeners[eventType] = append(s.listeners[eventType], callback)
}

func (s *Syncer) OnSync(callback mautrix.SyncHandler) {
	s.syncListeners = append(s.syncListeners, callback)
}

func (s *Syncer) OnEvent(callback mautrix.EventHandler) {
	s.globalListeners = append(s.globalListeners, callback)
}

// OnFailedSync always returns a 10 second wait period between failed /syncs, never a fatal error.
func (s *Syncer) OnFailedSync(res *mautrix.RespSync, err error) (time.Duration, error) {
	return 10 * time.Second, nil
}

func (s *Syncer) GetFilterJSON(id.UserID) *mautrix.Filter {
	return &mautrix.Filter{
		Room: mautrix.RoomFilter{
			Rooms: permittedRooms,
			Timeline: mautrix.FilterPart{
				Types: []event.Type{
					event.EventMessage,
					event.StateMember,
					event.StatePolicyServer,
					event.StatePolicyUser,
					event.StateTombstone,
				},
			},
		},
	}
}

var (
	errProcessPanic = errors.New("ProcessResponse panicked")
)
