// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperator

import (
	"fmt"

	"github.com/juju/errors"
	"gopkg.in/juju/charm.v6-unstable"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/api/common"
	apiwatcher "github.com/juju/juju/api/watcher"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/status"
	"github.com/juju/juju/watcher"
)

// CAASUnit represents individual CAAS units as seen by a caasoperator worker.
type CAASUnit struct {
	st   *State
	tag  names.UnitTag
	life params.Life
}

// Tag returns the caasunit's tag.
func (op *CAASUnit) Tag() names.UnitTag {
	return op.tag
}

// Name returns the name of the caasunit.
func (op *CAASUnit) Name() string {
	return op.tag.Id()
}

// String returns the caasunit as a string.
func (op *CAASUnit) String() string {
	return op.Name()
}

// Life returns the caasunit's lifecycle value.
func (op *CAASUnit) Life() params.Life {
	return op.life
}

// Refresh updates the cached local copy of the caasunit's data.
func (op *CAASUnit) Refresh() error {
	life, err := op.st.life(op.tag)
	if err != nil {
		return err
	}
	op.life = life
	return nil
}

// SetCAASUnitStatus sets the status of the caasunit.
func (op *CAASUnit) SetCAASUnitStatus(caasunitStatus status.Status, info string, data map[string]interface{}) error {
	if op.st.facade.BestAPIVersion() < 2 {
		return errors.NotImplementedf("SetCAASUnitStatus")
	}
	var result params.ErrorResults
	args := params.SetStatus{
		Entities: []params.EntityStatusArgs{
			{Tag: op.tag.String(), Status: caasunitStatus.String(), Info: info, Data: data},
		},
	}
	err := op.st.facade.FacadeCall("SetCAASUnitStatus", args, &result)
	if err != nil {
		return errors.Trace(err)
	}
	return result.OneError()
}

// CAASUnitStatus gets the status details of the caasunit.
func (op *CAASUnit) CAASUnitStatus() (params.StatusResult, error) {
	var results params.StatusResults
	args := params.Entities{
		Entities: []params.Entity{
			{Tag: op.tag.String()},
		},
	}
	err := op.st.facade.FacadeCall("CAASUnitStatus", args, &results)
	if err != nil {
		return params.StatusResult{}, errors.Trace(err)
	}
	if len(results.Results) != 1 {
		panic(errors.Errorf("expected 1 result, got %d", len(results.Results)))
	}
	result := results.Results[0]
	if result.Error != nil {
		return params.StatusResult{}, result.Error
	}
	return result, nil
}

// SetAgentStatus sets the status of the caasunit agent.
func (op *CAASUnit) SetAgentStatus(agentStatus status.Status, info string, data map[string]interface{}) error {
	var result params.ErrorResults
	args := params.SetStatus{
		Entities: []params.EntityStatusArgs{
			{Tag: op.tag.String(), Status: agentStatus.String(), Info: info, Data: data},
		},
	}
	setStatusFacadeCall := "SetAgentStatus"
	if op.st.facade.BestAPIVersion() < 2 {
		setStatusFacadeCall = "SetStatus"
	}
	err := op.st.facade.FacadeCall(setStatusFacadeCall, args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

// AddMetrics adds the metrics for the caasunit.
func (op *CAASUnit) AddMetrics(metrics []params.Metric) error {
	var result params.ErrorResults
	args := params.MetricsParams{
		Metrics: []params.MetricsParam{{
			Tag:     op.tag.String(),
			Metrics: metrics,
		}},
	}
	err := op.st.facade.FacadeCall("AddMetrics", args, &result)
	if err != nil {
		return errors.Annotate(err, "unable to add metric")
	}
	return result.OneError()
}

// AddMetricsBatches makes an api call to the caasuniterator requesting it to store metrics batches in state.
func (op *CAASUnit) AddMetricBatches(batches []params.MetricBatch) (map[string]error, error) {
	p := params.MetricBatchParams{
		Batches: make([]params.MetricBatchParam, len(batches)),
	}

	batchResults := make(map[string]error, len(batches))

	for i, batch := range batches {
		p.Batches[i].Tag = op.tag.String()
		p.Batches[i].Batch = batch

		batchResults[batch.UUID] = nil
	}
	results := new(params.ErrorResults)
	err := op.st.facade.FacadeCall("AddMetricBatches", p, results)
	if err != nil {
		return nil, errors.Annotate(err, "failed to send metric batches")
	}
	for i, result := range results.Results {
		batchResults[batches[i].UUID] = result.Error
	}
	return batchResults, nil
}

// EnsureDead sets the caasunit lifecycle to Dead if it is Alive or
// Dying. It does nothing otherwise.
func (op *CAASUnit) EnsureDead() error {
	var result params.ErrorResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("EnsureDead", args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

// Watch returns a watcher for observing changes to the caasunit.
func (op *CAASUnit) Watch() (watcher.NotifyWatcher, error) {
	return common.Watch(op.st.facade, op.tag)
}

// Service returns the service.
func (op *CAASUnit) CAASApplication() (*CAASApplication, error) {
	app := &CAASApplication{
		st:  op.st,
		tag: op.ApplicationTag(),
	}
	// Call Refresh() immediately to get the up-to-date
	// life and other needed locally cached fields.
	err := app.Refresh()
	if err != nil {
		return nil, err
	}
	return app, nil
}

// ConfigSettings returns the complete set of service charm config settings
// available to the caasunit. Unset values will be replaced with the default
// value for the associated option, and may thus be nil when no default is
// specified.
func (op *CAASUnit) ConfigSettings() (charm.Settings, error) {
	var results params.ConfigSettingsResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("ConfigSettings", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	return charm.Settings(result.Settings), nil
}

// ApplicationName returns the application name.
func (op *CAASUnit) ApplicationName() string {
	application, err := names.UnitApplication(op.Name())
	if err != nil {
		panic(err)
	}
	return application
}

// ApplicationTag returns the service tag.
func (op *CAASUnit) ApplicationTag() names.ApplicationTag {
	return names.NewApplicationTag(op.ApplicationName())
}

// Destroy, when called on a Alive caasunit, advances its lifecycle as far as
// possible; it otherwise has no effect. In most situations, the caasunit's
// life is just set to Dying; but if a principal caasunit that is not assigned
// to a provisioned machine is Destroyed, it will be removed from state
// directly.
func (op *CAASUnit) Destroy() error {
	var result params.ErrorResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("Destroy", args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

// Resolved returns the resolved mode for the caasunit.
//
// NOTE: This differs from state.CAASUnit.Resolved() by returning an
// error as well, because it needs to make an API call
func (op *CAASUnit) Resolved() (params.ResolvedMode, error) {
	var results params.ResolvedModeResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("Resolved", args, &results)
	if err != nil {
		return "", err
	}
	if len(results.Results) != 1 {
		return "", fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return "", result.Error
	}
	return result.Mode, nil
}

// PublicAddress returns the public address of the caasunit and whether it
// is valid.
//
// NOTE: This differs from state.CAASUnit.PublicAddres() by returning
// an error instead of a bool, because it needs to make an API call.
//
// TODO(dimitern): We might be able to drop this, once we have machine
// addresses implemented fully. See also LP bug 1221798.
func (op *CAASUnit) PublicAddress() (string, error) {
	var results params.StringResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("PublicAddress", args, &results)
	if err != nil {
		return "", err
	}
	if len(results.Results) != 1 {
		return "", fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return "", result.Error
	}
	return result.Result, nil
}

// PrivateAddress returns the private address of the caasunit and whether
// it is valid.
//
// NOTE: This differs from state.CAASUnit.PrivateAddress() by returning
// an error instead of a bool, because it needs to make an API call.
//
// TODO(dimitern): We might be able to drop this, once we have machine
// addresses implemented fully. See also LP bug 1221798.
func (op *CAASUnit) PrivateAddress() (string, error) {
	var results params.StringResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("PrivateAddress", args, &results)
	if err != nil {
		return "", err
	}
	if len(results.Results) != 1 {
		return "", fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return "", result.Error
	}
	return result.Result, nil
}

// OpenPorts sets the policy of the port range with protocol to be
// opened.
func (op *CAASUnit) OpenPorts(protocol string, fromPort, toPort int) error {
	var result params.ErrorResults
	args := params.EntitiesPortRanges{
		Entities: []params.EntityPortRange{{
			Tag:      op.tag.String(),
			Protocol: protocol,
			FromPort: fromPort,
			ToPort:   toPort,
		}},
	}
	err := op.st.facade.FacadeCall("OpenPorts", args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

// ClosePorts sets the policy of the port range with protocol to be
// closed.
func (op *CAASUnit) ClosePorts(protocol string, fromPort, toPort int) error {
	var result params.ErrorResults
	args := params.EntitiesPortRanges{
		Entities: []params.EntityPortRange{{
			Tag:      op.tag.String(),
			Protocol: protocol,
			FromPort: fromPort,
			ToPort:   toPort,
		}},
	}
	err := op.st.facade.FacadeCall("ClosePorts", args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

var ErrNoCharmURLSet = errors.New("caasunit has no charm url set")

// CharmURL returns the charm URL this caasunit is currently using.
//
// NOTE: This differs from state.CAASUnit.CharmURL() by returning
// an error instead of a bool, because it needs to make an API call.
func (op *CAASUnit) CharmURL() (*charm.URL, error) {
	var results params.StringBoolResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("CharmURL", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	if result.Result != "" {
		curl, err := charm.ParseURL(result.Result)
		if err != nil {
			return nil, err
		}
		return curl, nil
	}
	return nil, ErrNoCharmURLSet
}

// SetCharmURL marks the caasunit as currently using the supplied charm URL.
// An error will be returned if the caasunit is dead, or the charm URL not known.
func (op *CAASUnit) SetCharmURL(curl *charm.URL) error {
	if curl == nil {
		return fmt.Errorf("charm URL cannot be nil")
	}
	var result params.ErrorResults
	args := params.EntitiesCharmURL{
		Entities: []params.EntityCharmURL{
			{Tag: op.tag.String(), CharmURL: curl.String()},
		},
	}
	err := op.st.facade.FacadeCall("SetCharmURL", args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

// ClearResolved removes any resolved setting on the caasunit.
func (op *CAASUnit) ClearResolved() error {
	var result params.ErrorResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("ClearResolved", args, &result)
	if err != nil {
		return err
	}
	return result.OneError()
}

// WatchConfigSettings returns a watcher for observing changes to the
// caasunit's service configuration settings. The caasunit must have a charm URL
// set before this method is called, and the returned watcher will be
// valid only while the caasunit's charm URL is not changed.
func (op *CAASUnit) WatchConfigSettings() (watcher.NotifyWatcher, error) {
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("WatchConfigSettings", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewNotifyWatcher(op.st.facade.RawAPICaller(), result)
	return w, nil
}

// WatchAddresses returns a watcher for observing changes to the
// caasunit's addresses. The caasunit must be assigned to a machine before
// this method is called, and the returned watcher will be valid only
// while the caasunit's assigned machine is not changed.
func (op *CAASUnit) WatchAddresses() (watcher.NotifyWatcher, error) {
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("WatchCAASUnitAddresses", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewNotifyWatcher(op.st.facade.RawAPICaller(), result)
	return w, nil
}

// WatchActionNotifications returns a StringsWatcher for observing the
// ids of Actions added to the CAASUnit. The initial event will contain the
// ids of any Actions pending at the time the Watcher is made.
func (op *CAASUnit) WatchActionNotifications() (watcher.StringsWatcher, error) {
	var results params.StringsWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("WatchActionNotifications", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewStringsWatcher(op.st.facade.RawAPICaller(), result)
	return w, nil
}

// RequestReboot sets the reboot flag for its machine agent
func (op *CAASUnit) RequestReboot() error {
	// TODO MMCC: no assigned machine for caas units, need to handle reboot ID
	return nil
}

// MeterStatus returns the meter status of the caasunit.
func (op *CAASUnit) MeterStatus() (statusCode, statusInfo string, rErr error) {
	var results params.MeterStatusResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("GetMeterStatus", args, &results)
	if err != nil {
		return "", "", errors.Trace(err)
	}
	if len(results.Results) != 1 {
		return "", "", errors.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return "", "", errors.Trace(result.Error)
	}
	return result.Code, result.Info, nil
}

// WatchMeterStatus returns a watcher for observing changes to the
// caasunit's meter status.
func (op *CAASUnit) WatchMeterStatus() (watcher.NotifyWatcher, error) {
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: op.tag.String()}},
	}
	err := op.st.facade.FacadeCall("WatchMeterStatus", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewNotifyWatcher(op.st.facade.RawAPICaller(), result)
	return w, nil
}

// JoinedRelations returns the tags of the relations the application's units have joined.
func (u *CAASUnit) JoinedRelations() ([]names.RelationTag, error) {
	var results params.StringsResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: u.tag.String()}},
	}
	err := u.st.facade.FacadeCall("JoinedRelations", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	var relTags []names.RelationTag
	for _, rel := range result.Result {
		tag, err := names.ParseRelationTag(rel)
		if err != nil {
			return nil, err
		}
		relTags = append(relTags, tag)
	}
	return relTags, nil
}
