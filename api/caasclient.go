// Copyright 2013, 2014, 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package api

import (
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/apiserver/params"
)

// CAASClient represents the CAAS client-accessible part of the state.
type CAASClient struct {
	base.ClientFacade
	facade base.FacadeCaller
	st     *state
}

// Status returns the status of the Juju CAAS model.
func (c *CAASClient) Status(patterns []string) (*params.CAASStatus, error) {
	var result params.CAASStatus
	p := params.StatusParams{Patterns: patterns}
	if err := c.facade.FacadeCall("Status", p, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
