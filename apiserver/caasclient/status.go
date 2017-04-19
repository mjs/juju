// Copyright 2013, 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasclient

import (
	"sort"

	"github.com/juju/errors"
	"gopkg.in/juju/charm.v6-unstable"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/state"
	"github.com/juju/juju/status"
)

// Status gives the information needed for juju CAAS status over the API.
func (c *Client) Status(args params.StatusParams) (params.CAASStatus, error) {
	if err := c.checkCanRead(); err != nil {
		return params.CAASStatus{}, err
	}

	var noStatus params.CAASStatus
	var context statusContext
	var err error
	if context.applications, context.units, context.latestCharms, err =
		fetchAllApplicationsAndUnits(c.api.state, len(args.Patterns) <= 0); err != nil {
		return noStatus, errors.Annotate(err, "could not fetch applications and units")
	}

	logger.Debugf("Applications: %v", context.applications)

	modelStatus, err := c.modelStatus()
	if err != nil {
		return noStatus, errors.Annotate(err, "cannot determine model status")
	}
	return params.CAASStatus{
		Model:        modelStatus,
		Applications: context.processCAASApplications(),
	}, nil
}

func (c *Client) modelStatus() (params.CAASModelStatusInfo, error) {
	var info params.CAASModelStatusInfo

	m, err := c.api.state.CAASModel()
	if err != nil {
		return info, errors.Annotate(err, "cannot get model")
	}
	info.Name = m.Name()

	return info, nil
}

type statusContext struct {
	applications map[string]*state.CAASApplication
	relations    map[string][]*state.Relation
	units        map[string]map[string]*state.CAASUnit
	latestCharms map[charm.URL]*state.Charm
	leaders      map[string]string
}

// fetchAllApplicationsAndUnits returns a map from application name to application,
// a map from application name to unit name to unit, and a map from base charm URL to latest URL.
func fetchAllApplicationsAndUnits(st *state.CAASState, matchAny bool) (map[string]*state.CAASApplication, map[string]map[string]*state.CAASUnit, map[charm.URL]*state.Charm, error) {
	appMap := make(map[string]*state.CAASApplication)
	unitMap := make(map[string]map[string]*state.CAASUnit)
	latestCharms := make(map[charm.URL]*state.Charm)

	applications, err := st.AllCAASApplications()
	if err != nil {
		return nil, nil, nil, err
	}
	for _, s := range applications {
		units, err := s.AllCAASUnits()
		if err != nil {
			return nil, nil, nil, err
		}
		appUnitMap := make(map[string]*state.CAASUnit)
		for _, u := range units {
			appUnitMap[u.Name()] = u
		}
		if matchAny || len(appUnitMap) > 0 {
			unitMap[s.Name()] = appUnitMap
			appMap[s.Name()] = s
			// Record the base URL for the application's charm so that
			// the latest store revision can be looked up.
			charmURL, _ := s.CharmURL()
			if charmURL.Schema == "cs" {
				latestCharms[*charmURL.WithRevision(-1)] = nil
			}
		}
	}
	for baseURL := range latestCharms {
		ch, err := st.LatestPlaceholderCharm(&baseURL)
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, nil, nil, err
		}
		latestCharms[baseURL] = ch
	}

	return appMap, unitMap, latestCharms, nil
}

func (context *statusContext) processCAASApplications() map[string]params.CAASApplicationStatus {
	caasApps := make(map[string]params.CAASApplicationStatus)
	for _, s := range context.applications {
		caasApps[s.Name()] = context.processCAASApplication(s)
	}
	return caasApps
}

func (context *statusContext) processCAASApplication(caasApp *state.CAASApplication) params.CAASApplicationStatus {
	caasAppCharm, _, err := caasApp.Charm()
	if err != nil {
		return params.CAASApplicationStatus{Err: common.ServerError(err)}
	}

	var processedStatus = params.CAASApplicationStatus{
		Charm: caasAppCharm.URL().String(),
		Life:  processLife(caasApp),
	}

	if latestCharm, ok := context.latestCharms[*caasAppCharm.URL().WithRevision(-1)]; ok && latestCharm != nil {
		if latestCharm.Revision() > caasAppCharm.URL().Revision {
			processedStatus.CanUpgradeTo = latestCharm.String()
		}
	}

	units := context.units[caasApp.Name()]
	versions := make([]status.StatusInfo, 0, len(units))
	for _, unit := range units {
		statuses, err := unit.WorkloadVersionHistory().StatusHistory(
			status.StatusHistoryFilter{Size: 1},
		)
		if err != nil {
			processedStatus.Err = common.ServerError(err)
			return processedStatus
		}
		// Even though we fully expect there to be historical values there,
		// even the first should be the empty string, the status history
		// collection is not added to in a transactional manner, so it may be
		// not there even though we'd really like it to be. Such is mongo.
		if len(statuses) > 0 {
			versions = append(versions, statuses[0])
		}
	}
	if len(versions) > 0 {
		sort.Sort(bySinceDescending(versions))
		processedStatus.WorkloadVersion = versions[0].Message
	}

	return processedStatus
}

type lifer interface {
	Life() state.Life
}

func processLife(entity lifer) string {
	if life := entity.Life(); life != state.Alive {
		// alive is the usual state so omit it by default.
		return life.String()
	}
	return ""
}

type bySinceDescending []status.StatusInfo

// Len implements sort.Interface.
func (s bySinceDescending) Len() int { return len(s) }

// Swap implements sort.Interface.
func (s bySinceDescending) Swap(a, b int) { s[a], s[b] = s[b], s[a] }

// Less implements sort.Interface.
func (s bySinceDescending) Less(a, b int) bool { return s[a].Since.After(*s[b].Since) }
