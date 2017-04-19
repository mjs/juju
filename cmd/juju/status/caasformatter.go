// Copyright 2015, 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package status

import (
	"gopkg.in/juju/charm.v6-unstable"

	"github.com/juju/juju/apiserver/params"
)

type caasStatusFormatter struct {
	status         *params.CAASStatus
	controllerName string
	relations      map[int]params.RelationStatus
	isoTime        bool
}

// NewStatusFormatter takes stored model information (params.FullStatus) and populates
// the statusFormatter struct used in various status formatting methods
func NewCAASStatusFormatter(status *params.CAASStatus, isoTime bool) *caasStatusFormatter {
	return newCAASStatusFormatter(status, "", isoTime)
}

func newCAASStatusFormatter(status *params.CAASStatus, controllerName string, isoTime bool) *caasStatusFormatter {
	csf := &caasStatusFormatter{
		status:         status,
		controllerName: controllerName,
		relations:      make(map[int]params.RelationStatus),
		isoTime:        isoTime,
	}
	for _, relation := range status.Relations {
		csf.relations[relation.Id] = relation
	}
	return csf
}

func (csf *caasStatusFormatter) format() (formattedStatus, error) {
	if csf.status == nil {
		return formattedStatus{}, nil
	}
	model := csf.status.Model
	out := &formattedCAASStatus{
		Model: modelStatus{
			Name:             model.Name,
			Controller:       csf.controllerName,
			CloudRegion:      model.CloudRegion,
			Version:          model.Version,
			AvailableVersion: model.AvailableVersion,
		},
		Applications: make(map[string]caasApplicationStatus),
	}
	for sn, s := range csf.status.Applications {
		out.Applications[sn] = csf.formatCAASApplication(sn, s)
	}
	return formattedStatus{caasStatus: out}, nil
}

func (csf *caasStatusFormatter) formatCAASApplication(name string, caasApp params.CAASApplicationStatus) caasApplicationStatus {
	var (
		charmOrigin = ""
		charmName   = ""
		charmRev    = 0
	)
	if curl, err := charm.ParseURL(caasApp.Charm); err != nil {
		// We should never fail to parse a charm url sent back
		// but if we do, don't crash.
		logger.Errorf("failed to parse charm: %v", err)
	} else {
		switch curl.Schema {
		case "cs":
			charmOrigin = "jujucharms"
		case "local":
			charmOrigin = "local"
		default:
			charmOrigin = "unknown"
		}
		charmName = curl.Name
		charmRev = curl.Revision
	}

	out := caasApplicationStatus{
		Err:          caasApp.Err,
		Charm:        caasApp.Charm,
		CharmOrigin:  charmOrigin,
		CharmName:    charmName,
		CharmRev:     charmRev,
		Life:         caasApp.Life,
		Relations:    caasApp.Relations,
		CanUpgradeTo: caasApp.CanUpgradeTo,
		Units:        make(map[string]caasUnitStatus),
		Version:      caasApp.WorkloadVersion,
	}
	return out
}
