// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"time"

	"github.com/juju/errors"
	"gopkg.in/juju/worker.v1"

	"github.com/juju/juju/state/watcher"
)

// caasWorkers runs the workers that a State instance requires.
// It wraps a Runner instance which restarts any of the
// workers when they fail.
type caasWorkers struct {
	*worker.Runner
}

func newCAASWorkers(st *CAASState) (*caasWorkers, error) {
	ws := &caasWorkers{
		Runner: worker.NewRunner(worker.RunnerParams{
			IsFatal:      func(error) bool { return false },
			RestartDelay: time.Second,
			Clock:        st.clock,
		}),
	}
	ws.StartWorker(txnLogWorker, func() (worker.Worker, error) {
		txns := st.session.DB(jujuDB).C(txnLogC)
		return watcher.New(txns), nil
	})
	return ws, nil
}

func (ws *caasWorkers) txnLogWatcher() *watcher.Watcher {
	w, err := ws.Worker(txnLogWorker, nil)
	if err != nil {
		return watcher.NewDead(errors.Trace(err))
	}
	return w.(*watcher.Watcher)
}
