// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasapplication

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/charm.v6-unstable"
	csparams "gopkg.in/juju/charmrepo.v2-unstable/csclient/params"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/permission"
	"github.com/juju/juju/state"
)

var logger = loggo.GetLogger("juju.apiserver.caasapplication")

// NewFacade returns a new CAAS application facade.
func NewFacade(ctx facade.Context) (*Facade, error) {
	auth := ctx.Auth()
	if !auth.AuthClient() {
		return nil, common.ErrPerm
	}
	return &Facade{
		authorizer: auth,
		backend:    ctx.CAASState(),
	}, nil
}

// Facade implements the application interface and is the concrete
// implementation of the api end point.
type Facade struct {
	authorizer facade.Authorizer
	backend    *state.CAASState
}

func (facade *Facade) checkPermission(tag names.Tag, perm permission.Access) error {
	allowed, err := facade.authorizer.HasPermission(perm, tag)
	if err != nil {
		return errors.Trace(err)
	}
	if !allowed {
		return common.ErrPerm
	}
	return nil
}

func (facade *Facade) checkCanRead() error {
	return facade.checkPermission(facade.backend.ModelTag(), permission.ReadAccess)
}

func (facade *Facade) checkCanWrite() error {
	return facade.checkPermission(facade.backend.ModelTag(), permission.WriteAccess)
}

// Deploy fetches the charms from the charm store and deploys them
// using the specified placement directives.
// XXX CAAS - this is missing authentication checks (prototype)
func (facade *Facade) Deploy(args params.CAASApplicationsDeploy) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Applications)),
	}
	for i, arg := range args.Applications {
		err := deployApplication(facade.backend, arg)
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

func deployApplication(backend *state.CAASState, args params.CAASApplicationDeploy) error {
	curl, err := charm.ParseURL(args.CharmURL)
	if err != nil {
		return errors.Trace(err)
	}
	if curl.Revision < 0 {
		return errors.Errorf("charm url must include revision")
	}

	ch, err := backend.Charm(curl)
	if err != nil {
		return errors.Trace(err)
	}

	if err := checkMinVersion(ch); err != nil {
		return errors.Trace(err)
	}

	var settings charm.Settings
	if len(args.ConfigYAML) > 0 {
		settings, err = ch.Config().ParseSettingsYAML([]byte(args.ConfigYAML), args.ApplicationName)
		if err != nil {
			return errors.Trace(err)
		}
	}

	_, err = backend.AddCAASApplication(state.AddCAASApplicationArgs{
		Name:     args.ApplicationName,
		Charm:    ch,
		Channel:  csparams.Channel(args.Channel),
		Settings: settings,
		NumUnits: args.NumUnits,
	})
	return errors.Trace(err)
}

// DestroyApplication removes a given set of applications.
func (facade *Facade) DestroyApplication(args params.Entities) (params.DestroyApplicationResults, error) {
	if err := facade.checkCanWrite(); err != nil {
		return params.DestroyApplicationResults{}, err
	}
	destroyApp := func(entity params.Entity) (*params.DestroyApplicationInfo, error) {
		tag, err := names.ParseApplicationTag(entity.Tag)
		if err != nil {
			return nil, err
		}
		app, err := facade.backend.CAASApplication(tag.Id())
		if err != nil {
			return nil, err
		}
		units, err := app.AllCAASUnits()
		if err != nil {
			return nil, err
		}
		if err := app.Destroy(); err != nil {
			return nil, err
		}
		var info params.DestroyApplicationInfo
		for _, unit := range units {
			info.DestroyedUnits = append(info.DestroyedUnits, params.Entity{unit.UnitTag().String()})
		}
		return &info, nil
	}
	results := make([]params.DestroyApplicationResult, len(args.Entities))
	for i, entity := range args.Entities {
		info, err := destroyApp(entity)
		if err != nil {
			results[i].Error = common.ServerError(err)
			continue
		}
		results[i].Info = info
	}
	return params.DestroyApplicationResults{results}, nil
}

// AddUnits adds a given number of units to an application.
func (facade *Facade) AddUnits(args params.AddApplicationUnits) (params.AddApplicationUnitsResults, error) {
	units, err := addApplicationUnits(facade.backend, args)
	if err != nil {
		return params.AddApplicationUnitsResults{}, errors.Trace(err)
	}
	unitNames := make([]string, len(units))
	for i, unit := range units {
		unitNames[i] = unit.Name()
	}
	return params.AddApplicationUnitsResults{Units: unitNames}, nil
}

// addApplicationUnits adds a given number of units to an application.
func addApplicationUnits(backend *state.CAASState, args params.AddApplicationUnits) ([]*state.CAASUnit, error) {
	application, err := backend.CAASApplication(args.ApplicationName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if args.NumUnits < 1 {
		return nil, errors.New("must add at least one unit")
	}
	out := make([]*state.CAASUnit, args.NumUnits)
	for i := 0; i < args.NumUnits; i++ {
		unit, err := application.AddCAASUnit()
		if err != nil {
			return nil, err
		}
		out[i] = unit
	}
	return out, nil
}
