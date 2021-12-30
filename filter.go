// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"encoding/json"

	"github.com/qua3k/gomatrix"
)

func (f *Fallacy) setupPurgeFilter() string {
	filter := gomatrix.FilterPart{
		LazyLoadMembers: true,
		NotTypes: []string{
			"m.room.create",
			"m.room.history_visibility",
			"m.room.join_rules",
			"m.room.member",
			"m.room.power_levels",
			"m.room.avatar",
			"m.room.name",
			"m.room.pinned_events",
			"m.room.topic",
			"m.room.retention",
			"m.room.tombstone",
		},
	}

	filterJSON, err := json.Marshal(filter)
	if err != nil {
		panic(err)
	}

	return string(filterJSON)
}
