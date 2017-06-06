// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for infos.

package common

import (
	"time"

	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/jujuclient"
)

// CAASModelInfo contains information about a CAAS model.
type CAASModelInfo struct {
	// ShortName is un-qualified model name.
	ShortName      string `json:"short-name" yaml:"short-name"`
	Name           string `json:"name" yaml:"name"`
	UUID           string `json:"model-uuid" yaml:"model-uuid"`
	ControllerUUID string `json:"controller-uuid" yaml:"controller-uuid"`
	ControllerName string `json:"controller-name" yaml:"controller-name"`
	Owner          string `json:"owner" yaml:"owner"`
}

// CAASModelInfoFromParams translates a params.CAASModelInfo to CAASModelInfo.
func CAASModelInfoFromParams(info params.CAASModelInfo, now time.Time) (CAASModelInfo, error) {
	ownerTag, err := names.ParseUserTag(info.OwnerTag)
	if err != nil {
		return CAASModelInfo{}, errors.Trace(err)
	}
	return CAASModelInfo{
		ShortName:      info.Name,
		Name:           jujuclient.JoinOwnerModelName(ownerTag, info.Name),
		UUID:           info.UUID,
		ControllerUUID: info.ControllerUUID,
		Owner:          ownerTag.Id(),
	}, nil
}
