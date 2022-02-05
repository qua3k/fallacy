// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"log"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

// setupPurgeFilter returns a RoomEventFilter adequate for fetching the events
// necessary to purge messages.
func setupPurgeFilter() mautrix.FilterPart {
	return mautrix.FilterPart{
		LazyLoadMembers: true,
		NotTypes: []event.Type{
			event.EventRedaction,
			// avoid fetching state events
			event.StateAliases,
			event.StateBridge,
			event.StateCanonicalAlias,
			event.StateCreate,
			event.StateEncryption,
			event.StateHalfShotBridge,
			event.StateHistoryVisibility,
			event.StateJoinRules,
			event.StateMember,
			event.StatePinnedEvents,
			event.StatePowerLevels,
			event.StateRoomAvatar,
			event.StateRoomName,
			event.StateSpaceChild,
			event.StateTombstone,
			event.StateTopic,
		},
	}
}

// SetupSyncFilter returns a Filter adequate for fetching the events
// necessary to sync messages.
func (f *Fallacy) SetupSyncFilter() (resp *mautrix.RespCreateFilter) {
	filter := mautrix.Filter{
		Room: mautrix.RoomFilter{
			State: mautrix.FilterPart{
				LazyLoadMembers: true,
				Types: []event.Type{
					event.StateMember,
					event.StatePolicyServer,
					event.StatePolicyUser,
					event.StateTombstone,
				},
			},
			Timeline: mautrix.FilterPart{
				Types: []event.Type{
					event.EventMessage,
				},
			},
		},
	}

	resp, err := f.Client.CreateFilter(&filter)
	if err != nil {
		log.Panicln(err)
	}
	return
}