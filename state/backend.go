// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/utils/clock"

	"github.com/juju/juju/state/storage"
	"github.com/juju/juju/state/watcher"
)

// modelBackend collects together some useful internal state methods for
// accessing mongo and mapping local and global ids to one another.
type modelBackend interface {
	// uuid returns the UUID for the model.
	uuid() string

	// docID generates a globally unique ID value where the model
	// UUID is prefixed to the localID.
	docID(string) string

	modelClock() clock.Clock

	// localID returns the local ID value by stripping
	// off the model UUID prefix if it is there.
	localID(string) string

	// strictLocalID returns the local id value by removing the
	// model UUID prefix.
	//
	// If there is no prefix matching the State's model, an error is
	// returned.
	strictLocalID(string) (string, error)

	db() Database

	txnLogWatcher() *watcher.Watcher

	newStorage() storage.Storage
}

func (st *State) uuid() string {
	return st.ModelUUID()
}

func (st *State) docID(localID string) string {
	return ensureModelUUID(st.ModelUUID(), localID)
}

func (st *State) localID(ID string) string {
	modelUUID, localID, ok := splitDocID(ID)
	if !ok || modelUUID != st.ModelUUID() {
		return ID
	}
	return localID
}

func (st *State) strictLocalID(ID string) (string, error) {
	modelUUID, localID, ok := splitDocID(ID)
	if !ok || modelUUID != st.ModelUUID() {
		return "", errors.Errorf("unexpected id: %#v", ID)
	}
	return localID, nil
}
