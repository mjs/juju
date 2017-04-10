// Copyright 2014, 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// Package application provides access to the application api facade.
// This facade contains api calls that are specific to applications.
// As a rule of thumb, if the argument for an api requires an application name
// and affects only that application then the call belongs here.
package caasapplication

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/charmstore"
)

var logger = loggo.GetLogger("juju.api.caasapplication")

// Client allows access to the service API end point.
type Client struct {
	base.ClientFacade
	st     base.APICallCloser
	facade base.FacadeCaller
}

// NewClient creates a new client for accessing the application api.
func NewClient(st base.APICallCloser) *Client {
	frontend, backend := base.NewClientFacade(st, "CAASApplication")
	return &Client{ClientFacade: frontend, st: st, facade: backend}
}

// ModelUUID returns the model UUID from the client connection.
func (c *Client) ModelUUID() string {
	tag, ok := c.st.ModelTag()
	if !ok {
		logger.Warningf("controller-only API connection has no model tag")
	}
	return tag.Id()
}

// DeployArgs holds the arguments to be sent to Client.ServiceDeploy.
type DeployArgs struct {

	// CharmID identifies the charm to deploy.
	CharmID charmstore.CharmID

	// ApplicationName is the name to give the application.
	ApplicationName string

	// Series to be used for the machine.
	Series string

	// NumUnits is the number of units to deploy.
	NumUnits int

	// ConfigYAML is a string that overrides the default config.yml.
	ConfigYAML string
}

// Deploy obtains the charm, either locally or from the charm store, and deploys
// it. Placement directives, if provided, specify the machine on which the charm
// is deployed.
func (c *Client) Deploy(args DeployArgs) error {
	deployArgs := params.ApplicationsDeploy{
		Applications: []params.ApplicationDeploy{{
			ApplicationName: args.ApplicationName,
			Series:          args.Series,
			CharmURL:        args.CharmID.URL.String(),
			Channel:         string(args.CharmID.Channel),
			NumUnits:        args.NumUnits,
			ConfigYAML:      args.ConfigYAML,
		}},
	}
	var results params.ErrorResults
	var err error
	err = c.facade.FacadeCall("Deploy", deployArgs, &results)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(results.OneError())
}
