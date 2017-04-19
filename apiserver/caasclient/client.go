// Copyright 2013, 2014, 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasclient

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/permission"
	"github.com/juju/juju/state"
)

var logger = loggo.GetLogger("juju.apiserver.caasclient")

type API struct {
	auth      facade.Authorizer
	resources facade.Resources
	state     *state.CAASState
	client    *Client
}

// Client serves client-specific API methods.
type Client struct {
	api *API
}

func (c *Client) checkCanRead() error {
	isAdmin, err := c.api.auth.HasPermission(permission.SuperuserAccess, c.api.state.ControllerTag())
	if err != nil {
		return errors.Trace(err)
	}

	canRead, err := c.api.auth.HasPermission(permission.ReadAccess, c.api.state.ModelTag())
	if err != nil {
		return errors.Trace(err)
	}
	if !canRead && !isAdmin {
		return common.ErrPerm
	}
	return nil
}

func (c *Client) checkCanWrite() error {
	isAdmin, err := c.api.auth.HasPermission(permission.SuperuserAccess, c.api.state.ControllerTag())
	if err != nil {
		return errors.Trace(err)
	}

	canWrite, err := c.api.auth.HasPermission(permission.WriteAccess, c.api.state.ModelTag())
	if err != nil {
		return errors.Trace(err)
	}
	if !canWrite && !isAdmin {
		return common.ErrPerm
	}
	return nil
}

// NewFacade provides the signature required for facade registration.
func NewFacade(ctx facade.Context) (*Client, error) {
	if !ctx.IsCAAS() {
		return nil, errors.New("not a CAAS state")
	}

	authorizer := ctx.Auth()
	if !authorizer.AuthClient() {
		return nil, common.ErrPerm
	}

	return &Client{
		api: &API{
			state: ctx.CAASState(),
			auth:  authorizer,
		},
	}, nil
}
