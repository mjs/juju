// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasapplication

import (
	"regexp"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/utils/featureflag"
	"gopkg.in/juju/charm.v6-unstable"
	csparams "gopkg.in/juju/charmrepo.v2-unstable/csclient/params"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/feature"
	"github.com/juju/juju/network"
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
		statePool:  ctx.StatePool(),
	}, nil
}

// Facade implements the application interface and is the concrete
// implementation of the api end point.
type Facade struct {
	authorizer facade.Authorizer
	backend    *state.CAASState
	statePool  *state.StatePool
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

// applicationUrlEndpointParse is used to split an application url and optional
// relation name into url and relation name.
var applicationUrlEndpointParse = regexp.MustCompile("(?P<url>.*[/.][^:]*)(:(?P<relname>.*)$)?")

// AddRelation adds a relation between the specified endpoints and returns the relation info.
func (facade *Facade) AddRelation(args params.AddRelation) (params.AddRelationResults, error) {
	/*if err := facade.check.ChangeAllowed(); err != nil {
		return params.AddRelationResults{}, errors.Trace(err)
	}*/
	if err := facade.checkCanWrite(); err != nil {
		return params.AddRelationResults{}, errors.Trace(err)
	}

	inEps, err := facade.backend.InferEndpoints(args.Endpoints...)
	if err != nil {
		return params.AddRelationResults{}, errors.Trace(err)
	}
	rel, err := facade.backend.AddRelation(inEps...)
	if err != nil {
		return params.AddRelationResults{}, errors.Trace(err)
	}

	outEps := make(map[string]params.CharmRelation)
	for _, inEp := range inEps {
		outEp, err := rel.Endpoint(inEp.ApplicationName)
		if err != nil {
			return params.AddRelationResults{}, errors.Trace(err)
		}
		outEps[inEp.ApplicationName] = params.CharmRelation{
			Name:      outEp.Relation.Name,
			Role:      string(outEp.Relation.Role),
			Interface: outEp.Relation.Interface,
			Optional:  outEp.Relation.Optional,
			Limit:     outEp.Relation.Limit,
			Scope:     string(outEp.Relation.Scope),
		}
	}
	return params.AddRelationResults{Endpoints: outEps}, nil
}

// Consume adds remote applications to the model without creating any
// relations.
func (facade *Facade) Consume(args params.ConsumeApplicationArgs) (params.ErrorResults, error) {
	var consumeResults params.ErrorResults
	if !featureflag.Enabled(feature.CrossModelRelations) {
		err := errors.Errorf(
			"set %q feature flag to enable consuming remote applications",
			feature.CrossModelRelations,
		)
		return consumeResults, err
	}
	if err := facade.checkCanWrite(); err != nil {
		return consumeResults, errors.Trace(err)
	}
	/*if err := facade.check.ChangeAllowed(); err != nil {
		return consumeResults, errors.Trace(err)
	}*/

	results := make([]params.ErrorResult, len(args.Args))
	for i, arg := range args.Args {
		err := facade.consumeOne(arg)
		results[i].Error = common.ServerError(err)
	}
	consumeResults.Results = results
	return consumeResults, nil
}

func (facade *Facade) consumeOne(arg params.ConsumeApplicationArg) error {
	sourceModelTag, err := names.ParseModelTag(arg.SourceModelTag)
	if err != nil {
		return errors.Trace(err)
	}

	appName := arg.ApplicationAlias
	if appName == "" {
		appName = arg.OfferName
	}
	_, err = facade.saveRemoteApplication(sourceModelTag, appName, arg.ApplicationOffer)
	return err
}

// saveRemoteApplication saves the details of the specified remote application and its endpoints
// to the state model so relations to the remote application can be created.
func (facade *Facade) saveRemoteApplication(
	sourceModelTag names.ModelTag,
	applicationName string,
	offer params.ApplicationOffer,
) (*state.RemoteApplication, error) {
	remoteEps := make([]charm.Relation, len(offer.Endpoints))
	for j, ep := range offer.Endpoints {
		remoteEps[j] = charm.Relation{
			Name:      ep.Name,
			Role:      ep.Role,
			Interface: ep.Interface,
			Limit:     ep.Limit,
			Scope:     ep.Scope,
		}
	}

	remoteSpaces := make([]*environs.ProviderSpaceInfo, len(offer.Spaces))
	for i, space := range offer.Spaces {
		remoteSpaces[i] = providerSpaceInfoFromParams(space)
	}

	// If the a remote application with the same name and endpoints from the same
	// source model already exists, we will use that one.
	remoteApp, err := facade.maybeUpdateExistingApplicationEndpoints(applicationName, sourceModelTag, remoteEps)
	if err == nil {
		return remoteApp, nil
	} else if !errors.IsNotFound(err) {
		return nil, errors.Trace(err)
	}

	return facade.backend.AddRemoteApplication(state.AddRemoteApplicationParams{
		Name:        applicationName,
		OfferName:   offer.OfferName,
		URL:         offer.OfferURL,
		SourceModel: sourceModelTag,
		Endpoints:   remoteEps,
		Spaces:      remoteSpaces,
		Bindings:    offer.Bindings,
	})
}

// providerSpaceInfoFromParams converts a params.RemoteSpace to the
// equivalent ProviderSpaceInfo.
func providerSpaceInfoFromParams(space params.RemoteSpace) *environs.ProviderSpaceInfo {
	result := &environs.ProviderSpaceInfo{
		CloudType:          space.CloudType,
		ProviderAttributes: space.ProviderAttributes,
		SpaceInfo: network.SpaceInfo{
			Name:       space.Name,
			ProviderId: network.Id(space.ProviderId),
		},
	}
	for _, subnet := range space.Subnets {
		resultSubnet := network.SubnetInfo{
			CIDR:              subnet.CIDR,
			ProviderId:        network.Id(subnet.ProviderId),
			ProviderNetworkId: network.Id(subnet.ProviderNetworkId),
			SpaceProviderId:   network.Id(subnet.ProviderSpaceId),
			VLANTag:           subnet.VLANTag,
			AvailabilityZones: subnet.Zones,
		}
		result.Subnets = append(result.Subnets, resultSubnet)
	}
	return result
}

// maybeUpdateExistingApplicationEndpoints looks for a remote application with the
// specified name and source model tag and tries to update its endpoints with the
// new ones specified. If the endpoints are compatible, the newly updated remote
// application is returned.
func (facade *Facade) maybeUpdateExistingApplicationEndpoints(
	applicationName string, sourceModelTag names.ModelTag, remoteEps []charm.Relation,
) (*state.RemoteApplication, error) {
	existingRemoteApp, err := facade.backend.RemoteApplication(applicationName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, errors.Trace(err)
	}
	if existingRemoteApp.SourceModel().Id() != sourceModelTag.Id() {
		return nil, errors.AlreadyExistsf("remote application called %q from a different model", applicationName)
	}
	newEpsMap := make(map[charm.Relation]bool)
	for _, ep := range remoteEps {
		newEpsMap[ep] = true
	}
	existingEps, err := existingRemoteApp.Endpoints()
	if err != nil {
		return nil, errors.Trace(err)
	}
	maybeSameEndpoints := len(newEpsMap) == len(existingEps)
	existingEpsByName := make(map[string]charm.Relation)
	for _, ep := range existingEps {
		existingEpsByName[ep.Name] = ep.Relation
		delete(newEpsMap, ep.Relation)
	}
	sameEndpoints := maybeSameEndpoints && len(newEpsMap) == 0
	if sameEndpoints {
		return existingRemoteApp, nil
	}

	// Gather the new endpoints. All new endpoints passed to AddEndpoints()
	// below must not have the same name as an existing endpoint.
	var newEps []charm.Relation
	for ep := range newEpsMap {
		// See if we are attempting to update endpoints with the same name but
		// different relation data.
		if existing, ok := existingEpsByName[ep.Name]; ok && existing != ep {
			return nil, errors.Errorf("conflicting endpoint %v", ep.Name)
		}
		newEps = append(newEps, ep)
	}

	if len(newEps) > 0 {
		// Update the existing remote app to have the new, additional endpoints.
		if err := existingRemoteApp.AddEndpoints(newEps); err != nil {
			return nil, errors.Trace(err)
		}
	}
	return existingRemoteApp, nil
}

// DestroyRelation removes the relation between the specified endpoints.
func (facade *Facade) DestroyRelation(args params.DestroyRelation) error {
	if err := facade.checkCanWrite(); err != nil {
		return err
	}
	/*if err := facade.check.RemoveAllowed(); err != nil {
		return errors.Trace(err)
	}*/
	eps, err := facade.backend.InferEndpoints(args.Endpoints...)
	if err != nil {
		return err
	}
	rel, err := facade.backend.EndpointsRelation(eps...)
	if err != nil {
		return err
	}
	return rel.Destroy()
}
