// Copyright 2012-2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperator

import (
	"github.com/juju/errors"
	corecharm "gopkg.in/juju/charm.v6-unstable"
	"gopkg.in/juju/charm.v6-unstable/hooks"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/worker/caasoperator/charm"
	"github.com/juju/juju/worker/caasoperator/hook"
	"github.com/juju/juju/worker/caasoperator/runner"
)

// operationCallbacks implements operation.Callbacks, and exists entirely to
// keep those methods off the CaasOperator itself.
type operationCallbacks struct {
	op *CaasOperator
}

// PrepareHook is part of the operation.Callbacks interface.
func (opc *operationCallbacks) PrepareHook(hi hook.Info) (string, error) {
	name := string(hi.Kind)
	switch {
	case hi.Kind.IsRelation():
		var err error
		name, err = opc.op.relations.PrepareHook(hi)
		if err != nil {
			return "", err
		}
	// case hi.Kind.IsStorage():
	// 	if err := opc.op.storage.ValidateHook(hi); err != nil {
	// 		return "", err
	// 	}
	// 	storageName, err := names.StorageName(hi.StorageId)
	// 	if err != nil {
	// 		return "", err
	// 	}
	// 	name = fmt.Sprintf("%s-%s", storageName, hi.Kind)
	// 	// TODO(axw) if the agent is not installed yet,
	// 	// set the status to "preparing storage".
	case hi.Kind == hooks.ConfigChanged:
		// TODO(axw)
		//opc.op.f.DiscardConfigEvent()
	}
	return name, nil
}

// CommitHook is part of the operation.Callbacks interface.
func (opc *operationCallbacks) CommitHook(hi hook.Info) error {
	switch {
	case hi.Kind.IsRelation():
		return opc.op.relations.CommitHook(hi)
		// case hi.Kind.IsStorage():
		// 	return opc.op.storage.CommitHook(hi)
	}
	return nil
}

func notifyHook(hook string, ctx runner.CAASContext, method func(string)) {
	// if r, err := ctx.HookRelation(); err == nil {
	// 	remote, _ := ctx.RemoteUnitName()
	// 	if remote != "" {
	// 		remote = " " + remote
	// 	}
	// 	hook = hook + remote + " " + r.FakeId()
	// }
	// method(hook)
}

// NotifyHookCompleted is part of the operation.Callbacks interface.
func (opc *operationCallbacks) NotifyHookCompleted(hook string, ctx runner.CAASContext) {
	if opc.op.observer != nil {
		notifyHook(hook, ctx, opc.op.observer.HookCompleted)
	}
}

// NotifyHookFailed is part of the operation.Callbacks interface.
func (opc *operationCallbacks) NotifyHookFailed(hook string, ctx runner.CAASContext) {
	if opc.op.observer != nil {
		notifyHook(hook, ctx, opc.op.observer.HookFailed)
	}
}

// FailAction is part of the operation.Callbacks interface.
func (opc *operationCallbacks) FailAction(actionId, message string) error {
	if !names.IsValidAction(actionId) {
		return errors.Errorf("invalid action id %q", actionId)
	}
	tag := names.NewActionTag(actionId)
	err := opc.op.st.ActionFinish(tag, params.ActionFailed, nil, message)
	if params.IsCodeNotFoundOrCodeUnauthorized(err) {
		err = nil
	}
	return err
}

// GetArchiveInfo is part of the operation.Callbacks interface.
func (opc *operationCallbacks) GetArchiveInfo(charmURL *corecharm.URL) (charm.BundleInfo, error) {
	ch, err := opc.op.st.Charm(charmURL)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ch, nil
}

// SetCurrentCharm is part of the operation.Callbacks interface.
func (opc *operationCallbacks) SetCurrentCharm(charmURL *corecharm.URL) error {
	return nil
	//XXX return opc.op.caasunit.SetCharmURL(charmURL)
}

// SetExecutingStatus is part of the operation.Callbacks interface.
func (opc *operationCallbacks) SetExecutingStatus(message string) error {
	return nil
	//XXX return setAgentStatus(opc.op, status.Executing, message, nil)
}
