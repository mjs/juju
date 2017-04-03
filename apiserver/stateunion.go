// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import "github.com/juju/juju/state"

type stateUnion struct {
	state     *state.State
	caasState *state.CAASState
}

func newStateUnion(st *state.State) *stateUnion {
	return &stateUnion{
		state: st,
	}
}

func newCAASStateUnion(st *state.CAASState) *stateUnion {
	return &stateUnion{
		caasState: st,
	}
}

func (s *stateUnion) State() *state.State {
	if s.caasState != nil {
		panic("caasState unexpectedly set")
	}
	return s.state
}

func (s *stateUnion) CAASState() *state.CAASState {
	if s.state != nil {
		panic("st unexpectedly set")
	}
	return s.caasState
}

func (s *stateUnion) IsIAAS() bool {
	return s.state != nil
}

func (s *stateUnion) IsCAAS() bool {
	return s.caasState != nil
}
