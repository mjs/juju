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

	// Watch returns a watcher that fires when the application's units change
	WatchUnits() (watcher.StringsWatcher, error)

	AllCAASUnits() ([]CAASUnit, error)

	// WatchRelation returns a watcher that fires when the relations on this
	// service change.
	WatchRelations() (watcher.StringsWatcher, error)
}

type CAASUnit interface {
	Tag() names.UnitTag
	Name() string
	Life() params.Life
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

type apiCAASApplication struct {
	*caasoperator.CAASApplication
}

type apiCAASUnit struct {
	*caasoperator.CAASUnit
}

type apiRelation struct {
	*caasoperator.Relation
}

func (st apiState) Relation(tag names.RelationTag) (Relation, error) {
	r, err := st.State.Relation(tag)
	return apiRelation{r}, err
}

func (app apiCAASApplication) AllCAASUnits() ([]CAASUnit, error) {
	units, err := app.AllCAASUnits()
	if err != nil {
		return nil, err
	}
	out := make([]CAASUnit, 0, len(units))
	for _, unit := range units {
		out = append(out, unit)
	}
	return out, nil
}

func (st apiState) CAASUnit(tag names.UnitTag) (CAASUnit, error) {
	u, err := st.State.CAASUnit(tag) // XXX todo multiple units
	return apiCAASUnit{u}, err
}

func (st apiState) CAASApplication(tag names.ApplicationTag) (CAASApplication, error) {
	s, err := st.State.CAASApplication(tag)
	return apiCAASApplication{s}, err
}

func (u apiCAASUnit) CAASApplication() (CAASApplication, error) {
	s, err := u.CAASUnit.CAASApplication()
	return apiCAASApplication{s}, err
}
