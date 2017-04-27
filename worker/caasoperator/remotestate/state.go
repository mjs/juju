// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package remotestate

import (
	"gopkg.in/juju/charm.v6-unstable"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/api/caasoperator"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/watcher"
)

type State interface {
	Relation(names.RelationTag) (Relation, error)
	// StorageAttachment(names.StorageTag, names.CaasUnit) (params.StorageAttachment, error)
	// StorageAttachmentLife([]params.StorageAttachmentId) ([]params.LifeResult, error)
	CAASUnit(names.UnitTag) (CAASUnit, error)
	CAASApplication(names.ApplicationTag) (CAASApplication, error)
	WatchRelationUnits(names.RelationTag, names.UnitTag) (watcher.RelationUnitsWatcher, error)
	// WatchStorageAttachment(names.StorageTag, names.UnitTag) (watcher.NotifyWatcher, error)
}

type CAASUnit interface {
	Life() params.Life
	Refresh() error
	Resolved() (params.ResolvedMode, error)
	CAASApplication() (CAASApplication, error)
	Tag() names.UnitTag
	Watch() (watcher.NotifyWatcher, error)
	WatchAddresses() (watcher.NotifyWatcher, error)
	WatchConfigSettings() (watcher.NotifyWatcher, error)
	WatchActionNotifications() (watcher.StringsWatcher, error)
}

type CAASApplication interface {
	// CharmModifiedVersion returns a revision number for the charm that
	// increments whenever the charm or a resource for the charm changes.
	CharmModifiedVersion() (int, error)
	// CharmURL returns the url for the charm for this service.
	CharmURL() (*charm.URL, bool, error)
	// Life returns whether the service is alive.
	Life() params.Life
	// Refresh syncs this value with the api server.
	Refresh() error
	// Tag returns the tag for this service.
	Tag() names.ApplicationTag
	// Watch returns a watcher that fires when this service changes.
	Watch() (watcher.NotifyWatcher, error)
	// WatchRelation returns a watcher that fires when the relations on this
	// service change.
	WatchRelations() (watcher.StringsWatcher, error)
}

type Relation interface {
	Id() int
	Life() params.Life
}

func NewAPIState(st *caasoperator.State) State {
	return apiState{st}
}

type apiState struct {
	*caasoperator.State
}

type apiCaasUnit struct {
	*caasoperator.CAASUnit
}

type apiService struct {
	*caasoperator.CAASApplication
}

type apiRelation struct {
	*caasoperator.Relation
}

func (st apiState) Relation(tag names.RelationTag) (Relation, error) {
	r, err := st.State.Relation(tag)
	return apiRelation{r}, err
}

func (st apiState) CAASUnit(tag names.UnitTag) (CAASUnit, error) {
	u, err := st.State.CAASUnit(tag) // XXX todo multiple units
	return apiCaasUnit{u}, err
}

func (st apiState) CAASApplication(tag names.ApplicationTag) (CAASApplication, error) {
	s, err := st.State.CAASApplication(tag)
	return apiService{s}, err
}

func (u apiCaasUnit) CAASApplication() (CAASApplication, error) {
	s, err := u.CAASUnit.CAASApplication()
	return apiService{s}, err
}
