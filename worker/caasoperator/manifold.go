// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperator

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/utils/clock"
	"gopkg.in/juju/names.v2"
	worker "gopkg.in/juju/worker.v1"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/caasoperator"
	"github.com/juju/juju/worker/caasoperator/operation"
	"github.com/juju/juju/worker/caasoperator/resolver"
	"github.com/juju/juju/worker/dependency"
	"github.com/juju/juju/worker/fortress"
)

// ManifoldConfig defines the names of the manifolds on which a
// Manifold will depend.
type ManifoldConfig struct {
	AgentName string
	//	APICallerName         string
	MachineLockName string
	Clock           clock.Clock
	//	LeadershipTrackerName string
	CharmDirName          string
	HookRetryStrategyName string
	TranslateResolverErr  func(error) error
}

// Manifold returns a dependency manifold that runs a caasoperator worker,
// using the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	logger.Infof("About to return manifold struct")
	return dependency.Manifold{
		Inputs: []string{
			config.AgentName,
			//config.APICallerName,
			//config.CharmDirName,
			//config.HookRetryStrategyName,
		},
		Start: func(context dependency.Context) (worker.Worker, error) {
			// MMCC debug
			rootlogger := loggo.GetLogger("")
			rootlogger.SetLogLevel(loggo.TRACE)
			// end

			logger.Errorf("In caasoperator manifold Start")
			if config.Clock == nil {
				return nil, errors.NotValidf("missing Clock")
			}
			if config.MachineLockName == "" {
				return nil, errors.NotValidf("missing MachineLockName")
			}
			logger.Errorf("collecting resources")
			// Collect all required resources.
			var agent agent.Agent
			if err := context.Get(config.AgentName, &agent); err != nil {
				return nil, err
			}
			var apiConn api.Connection
			// if err := context.Get(config.APICallerName, &apiConn); err != nil {
			// 	// TODO(fwereade): absence of an APICaller shouldn't be the end of
			// 	// the world -- we ought to return a type that can at least run the
			// 	// leader-deposed hook -- but that's not done yet.
			// 	return nil, err
			// }
			// var charmDirGuard fortress.Guard
			// if err := context.Get(config.CharmDirName, &charmDirGuard); err != nil {
			// 	return nil, err
			// }
			// logger.Errorf("getting retry strategy")
			// var hookRetryStrategy params.RetryStrategy
			// if err := context.Get(config.HookRetryStrategyName, &hookRetryStrategy); err != nil {
			// 	return nil, err
			// }

			//			downloader := api.NewCharmDownloader(apiConn.Client())
			logger.Errorf("getting agent config")
			manifoldConfig := config
			// Configure and start the caasoperator.
			agentConfig := agent.CurrentConfig()
			tag := agentConfig.Tag()
			caasoperatorTag, ok := tag.(names.ApplicationTag)
			if !ok {
				return nil, errors.Errorf("expected a caasoperator tag, got %v", tag)
			}
			caasoperatorFacade := caasoperator.NewState(apiConn, caasoperatorTag)
			logger.Errorf("creating caasoperator")
			caasoperator, err := NewCaasOperator(&CaasOperatorParams{
				CaasOperatorFacade: caasoperatorFacade,
				CaasOperatorTag:    caasoperatorTag,
				//				LeadershipTracker:    leadershipTracker,
				DataDir: agentConfig.DataDir(),
				//				Downloader:           downloader,
				MachineLockName: manifoldConfig.MachineLockName,
				//CharmDirGuard:        charmDirGuard,
				UpdateStatusSignal: NewUpdateStatusTimer(),
				//HookRetryStrategy:    hookRetryStrategy,
				NewOperationExecutor: operation.NewExecutor,
				TranslateResolverErr: config.TranslateResolverErr,
				Clock:                manifoldConfig.Clock,
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			return caasoperator, nil
		},
	}
}

// TranslateFortressErrors turns errors returned by dependent
// manifolds due to fortress lockdown (i.e. model migration) into an
// error which causes the resolver loop to be restarted. When this
// happens the caasoperator is about to be shut down anyway.
func TranslateFortressErrors(err error) error {
	if fortress.IsFortressError(err) {
		return resolver.ErrRestart
	}
	return err
}