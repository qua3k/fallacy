// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// stateType are the omitted state event types.
var stateType = []event.Type{
	event.EventRedaction,
	event.StateAliases,
	event.StateCanonicalAlias,
	event.StateCreate,
	event.StateJoinRules,
	event.StateHistoryVisibility,
	event.StateGuestAccess,
	event.StateMember,
	event.StatePowerLevels,
	event.StateRoomName,
	event.StateTopic,
	event.StateRoomAvatar,
	event.StatePinnedEvents,
	event.StateServerACL,
	event.StateTombstone,
	event.StatePolicyRoom,
	event.StatePolicyServer,
	event.StatePolicyUser,
	event.StateEncryption,
	event.StateBridge,
	event.StateHalfShotBridge,
	event.StateSpaceChild,
	event.StateSpaceParent,
}

// purgeFilter is the standard filter for purging messages, omitting state events.
var purgeFilter = &mautrix.FilterPart{LazyLoadMembers: true, NotTypes: stateType}

func userFilter(user id.UserID) mautrix.FilterPart {
	return mautrix.FilterPart{
		LazyLoadMembers: true,
		NotTypes:        stateType,
		Senders:         []id.UserID{user},
	}
}
