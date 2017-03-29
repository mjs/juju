// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"runtime/debug"
	"sync"

	"gopkg.in/juju/names.v2"

	"github.com/juju/errors"
)

// NewCAASStatePool returns a new CAASStatePool instance. It takes a State
// connected to the system (controller model).
func NewCAASStatePool(controllerState *State) *CAASStatePool {
	return &CAASStatePool{
		controllerState: controllerState,
		pool:            make(map[string]*CAASPoolItem),
	}
}

// CAASPoolItem holds a CAASState and tracks how many requests are using it
// and whether it's been marked for removal.
type CAASPoolItem struct {
	state            *CAASState
	remove           bool
	referenceSources map[uint64]string
}

func (i *CAASPoolItem) refCount() int {
	return len(i.referenceSources)
}

// CAASStatePool is a cache of CAASState instances for multiple
// models. Clients should call Release when they have finished with any
// state.
//
// XXX this is mostly just a copy of StatePool, adapted for
// CAASState. A better solution is probably to modify CAASState to
// support multiple State-like types (or at least extract some of the
// common functionality).
type CAASStatePool struct {
	controllerState *State
	// mu protects pool
	mu   sync.Mutex
	pool map[string]*CAASPoolItem
	// sourceKey is used to provide a unique number as a key for the
	// referencesSources structure in the pool.
	sourceKey uint64
}

// Get returns a CAASState for a given CAAS model from the pool,
// creating one if required. If the State has been marked for removal
// because there are outstanding uses, an error will be returned.
func (p *CAASStatePool) Get(modelUUID string) (*CAASState, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	item, ok := p.pool[modelUUID]
	if ok && item.remove {
		// We don't want to allow increasing the refcount of a model
		// that's been removed.
		return nil, nil, errors.Errorf("model %v has been removed", modelUUID)
	}

	p.sourceKey++
	key := p.sourceKey
	// released is here to be captured by the closure for the releaser.
	// This is to ensure that the releaser function can only be called once.
	released := false

	releaser := func() {
		if released {
			return
		}
		err := p.release(modelUUID, key)
		if err != nil {
			logger.Errorf("releasing state back to pool: %s", err.Error())
		}
		released = true
	}
	source := string(debug.Stack())

	if ok {
		item.referenceSources[key] = source
		return item.state, releaser, nil
	}

	st, err := p.controllerState.ForCAASModel(names.NewModelTag(modelUUID))
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to create state for model %v", modelUUID)
	}
	p.pool[modelUUID] = &CAASPoolItem{
		state: st,
		referenceSources: map[uint64]string{
			key: source,
		},
	}
	return st, releaser, nil
}

// release indicates that the client has finished using the State. If the
// state has been marked for removal, it will be closed and removed
// when the final Release is done.
func (p *CAASStatePool) release(modelUUID string, key uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	item, ok := p.pool[modelUUID]
	if !ok {
		return errors.Errorf("unable to return unknown model %v to the pool", modelUUID)
	}
	if item.refCount() == 0 {
		return errors.Errorf("state pool refcount for model %v is already 0", modelUUID)
	}
	delete(item.referenceSources, key)
	return p.maybeRemoveItem(modelUUID, item)
}

// Remove takes the state out of the pool and closes it, or marks it
// for removal if it's currently being used (indicated by Gets without
// corresponding Releases).
func (p *CAASStatePool) Remove(modelUUID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	item, ok := p.pool[modelUUID]
	if !ok {
		// Don't require the client to keep track of what we've seen -
		// ignore unknown model uuids.
		return nil
	}
	item.remove = true
	return p.maybeRemoveItem(modelUUID, item)
}

func (p *CAASStatePool) maybeRemoveItem(modelUUID string, item *CAASPoolItem) error {
	if item.remove && item.refCount() == 0 {
		delete(p.pool, modelUUID)
		return item.state.Close()
	}
	return nil
}

// Close closes all State instances in the pool.
func (p *CAASStatePool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for _, item := range p.pool {
		if item.refCount() != 0 || item.remove {
			logger.Warningf(
				"state for %v leaked from pool - references: %v, removed: %v",
				item.state.ModelUUID(),
				item.refCount(),
				item.remove,
			)
		}
		err := item.state.Close()
		if err != nil {
			lastErr = err
		}
	}
	p.pool = make(map[string]*CAASPoolItem)
	return errors.Annotate(lastErr, "at least one error closing a state")
}
