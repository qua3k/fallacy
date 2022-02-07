// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var purgeFilter = mautrix.FilterPart{
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

func purgeUserFilter(users []string) mautrix.FilterPart {
	userIDs := make([]id.UserID, len(users))
	for i, u := range users {
		userIDs[i] = id.UserID(u)
	}

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
		Senders: userIDs,
	}
}
