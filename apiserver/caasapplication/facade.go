// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasapplication

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/charm.v6-unstable"
	csparams "gopkg.in/juju/charmrepo.v2-unstable/csclient/params"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/state"
)

var logger = loggo.GetLogger("juju.apiserver.caasapplication")

// NewFacade returns a new CAAS application facade.
func NewFacade(ctx facade.Context) (*Facade, error) {
	if !ctx.Auth().AuthClient() {
		return nil, common.ErrPerm
	}
	return &Facade{
		backend: ctx.CAASState(),
	}, nil
}

// Facade implements the application interface and is the concrete
// implementation of the api end point.
type Facade struct {
	backend *state.CAASState
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

	/* XXX needs charm first
	var settings charm.Settings
	settings, err = ch.Config().ParseSettingsYAML([]byte(args.ConfigYAML), args.ApplicationName)
	if err != nil {
		return errors.Trace(err)
	}
	*/

	_, err = backend.AddCAASApplication(state.AddCAASApplicationArgs{
		Name: args.ApplicationName,
		// Charm:  XXX needs to exist and be loaded in state first,
		Channel: csparams.Channel(args.Channel),
		// XXX Settings: settings,
	})
	return errors.Trace(err)
}
