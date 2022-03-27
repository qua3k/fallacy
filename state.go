// Copyright 2021 The fallacy Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fallacy

import (
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// powerLevels returns a power levels struct from the specified roomID.
func powerLevels(roomID id.RoomID) (resp event.PowerLevelsEventContent, err error) {
	err = Client.StateEvent(roomID, event.StatePowerLevels, "", &resp)
	return
}

// acls returns an ACL struct.
func acls(roomID id.RoomID) (resp event.ServerACLEventContent, err error) {
	err = Client.StateEvent(roomID, event.StateServerACL, "", &resp)
	return
}

func roomName(roomID id.RoomID) (resp event.RoomNameEventContent, err error) {
	err = Client.StateEvent(roomID, event.StateRoomName, "", &resp)
	return
}
