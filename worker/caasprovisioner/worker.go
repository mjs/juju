// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasprovisioner

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/worker.v1"

	"github.com/juju/juju/worker/caasmodelworkermanager"
	"github.com/juju/juju/worker/catacomb"
)

var logger = loggo.GetLogger("juju.workers.caasprovisioner")

func New(newState caasmodelworkermanager.NewStateFunc) (worker.Worker, error) {
	p := &provisioner{
		newState: newState,
	}
	err := catacomb.Invoke(catacomb.Plan{
		Site: &p.catacomb,
		Work: p.loop,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return p, nil
}

type provisioner struct {
	catacomb catacomb.Catacomb
	newState caasmodelworkermanager.NewStateFunc
}

// Kill is part of the worker.Worker interface.
func (p *provisioner) Kill() {
	p.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (p *provisioner) Wait() error {
	return p.catacomb.Wait()
}

func (p *provisioner) loop() error {
	st, err := p.newState()
	if err != nil {
		return errors.Trace(err)
	}
	defer st.Close()

	w := st.WatchApplications()
	p.catacomb.Add(w)

	for {
		select {
		case apps := <-w.Changes():
			for _, app := range apps {
				// XXX do something useful here - will need to track
				// already done work (see how the pre-existing
				// provisioner does it)
				logger.Infof("saw app: %s", app)
			}
		case <-p.catacomb.Dying():
			return p.catacomb.ErrDying()
		}
	}
}
