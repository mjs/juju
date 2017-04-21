// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package agent

import (
	"runtime"
	"time"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	//"github.com/juju/loggo"
	"github.com/juju/utils/featureflag"
	"github.com/juju/utils/voyeur"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/juju/names.v2"
	worker "gopkg.in/juju/worker.v1"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/tomb.v1"

	"github.com/juju/juju/agent"
	//	apicaasoperator "github.com/juju/juju/api/caasoperator"
	"github.com/juju/juju/cmd/jujud/agent/caasoperator"
	cmdutil "github.com/juju/juju/cmd/jujud/util"
	jujuversion "github.com/juju/juju/version"
	jworker "github.com/juju/juju/worker"
	"github.com/juju/juju/worker/dependency"
	"github.com/juju/juju/worker/introspection"
	"github.com/juju/juju/worker/logsender"
)

var (

	// should be an explicit dependency, can't do it cleanly yet
	caasOperatorManifolds = caasoperator.Manifolds
)

// CaasOperatorAgent is a cmd.Command responsible for running a CAAS operator agent.
type CaasOperatorAgent struct {
	cmd.CommandBase
	tomb tomb.Tomb
	AgentConf
	configChangedVal *voyeur.Value
	ApplicationName  string
	runner           *worker.Runner
	bufferedLogger   *logsender.BufferedLogWriter
	setupLogging     func(agent.Config) error
	logToStdErr      bool
	ctx              *cmd.Context

	// Used to signal that the upgrade worker will not
	// reboot the agent on startup because there are no
	// longer any immediately pending agent upgrades.
	// Channel used as a selectable bool (closed means true).
	initialUpgradeCheckComplete chan struct{}

	prometheusRegistry *prometheus.Registry
}

func NewCaasOperatorAgent(ctx *cmd.Context, bufferedLogger *logsender.BufferedLogWriter) (*CaasOperatorAgent, error) {
	prometheusRegistry, err := newPrometheusRegistry()
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &CaasOperatorAgent{
		AgentConf:        NewAgentConf(""),
		configChangedVal: voyeur.NewValue(true),
		ctx:              ctx,
		initialUpgradeCheckComplete: make(chan struct{}),
		bufferedLogger:              bufferedLogger,
		prometheusRegistry:          prometheusRegistry,
	}, nil
}

func (op *CaasOperatorAgent) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "caasoperator",
		Purpose: "run a juju CAAS Operator",
	}
}

func (op *CaasOperatorAgent) SetFlags(f *gnuflag.FlagSet) {
	op.AgentConf.AddFlags(f)
	f.StringVar(&op.ApplicationName, "application-name", "", "name of the application")
	f.BoolVar(&op.logToStdErr, "log-to-stderr", false, "whether to log to standard error instead of log files")
}

// Init initializes the command for running.
func (op *CaasOperatorAgent) Init(args []string) error {
	if err := op.AgentConf.CheckArgs(args); err != nil {
		return err
	}
	op.runner = worker.NewRunner(worker.RunnerParams{IsFatal: cmdutil.IsFatal, MoreImportant: cmdutil.MoreImportant, RestartDelay: jworker.RestartDelay})

	if !op.logToStdErr {
		if err := op.ReadConfig(op.Tag().String()); err != nil {
			return err
		}
		operatorConfig := op.CurrentConfig()

		// the writer in ctx.stderr gets set as the loggo writer in github.com/juju/cmd/logging.go
		op.ctx.Stderr = &lumberjack.Logger{
			Filename:   agent.LogFilename(operatorConfig),
			MaxSize:    300, // megabytes
			MaxBackups: 2,
		}

	}

	return nil
}

func (op *CaasOperatorAgent) Stop() error {
	op.runner.Kill()
	return op.tomb.Wait()
}

func (op *CaasOperatorAgent) Run(ctx *cmd.Context) error {
	defer op.tomb.Done()
	logger.Infof("Starting Run()")
	if err := op.ReadConfig(op.Tag().String()); err != nil {
		return err
	}
	agentLogger.Infof("caas operator %v start (%s [%s])", op.Tag().String(), jujuversion.Current, runtime.Compiler)
	if flags := featureflag.String(); flags != "" {
		logger.Warningf("developer feature flags enabled: %s", flags)
	}

	op.runner.StartWorker("api", op.APIWorkers)
	err := cmdutil.AgentDone(logger, op.runner.Wait())
	op.tomb.Kill(err)
	return err
}

// APIWorkers returns a dependency.Engine running the operator's responsibilities.
func (op *CaasOperatorAgent) APIWorkers() (worker.Worker, error) {
	manifolds := caasOperatorManifolds(caasoperator.ManifoldsConfig{
		Agent:                agent.APIHostPortsSetter{op},
		LogSource:            op.bufferedLogger.Logs(),
		LeadershipGuarantee:  30 * time.Second,
		AgentConfigChanged:   op.configChangedVal,
		PrometheusRegisterer: op.prometheusRegistry,
	})

	config := dependency.EngineConfig{
		IsFatal:     cmdutil.IsFatal,
		WorstError:  cmdutil.MoreImportantError,
		ErrorDelay:  3 * time.Second,
		BounceDelay: 10 * time.Millisecond,
	}
	engine, err := dependency.NewEngine(config)
	if err != nil {
		return nil, err
	}
	if err := dependency.Install(engine, manifolds); err != nil {
		if err := worker.Stop(engine); err != nil {
			logger.Errorf("while stopping engine with bad manifolds: %v", err)
		}
		return nil, err
	}
	if err := startIntrospection(introspectionConfig{
		Agent:              op,
		Engine:             engine,
		NewSocketName:      DefaultIntrospectionSocketName,
		PrometheusGatherer: op.prometheusRegistry,
		WorkerFunc:         introspection.NewWorker,
	}); err != nil {
		// If the introspection worker failed to start, we just log error
		// but continue. It is very unlikely to happen in the real world
		// as the only issue is connecting to the abstract domain socket
		// and the agent is controlled by by the OS to only have one.
		logger.Errorf("failed to start introspection worker: %v", err)
	}
	return engine, nil
}

func (op *CaasOperatorAgent) Tag() names.Tag {
	return names.NewApplicationTag(op.ApplicationName)
}

func (op *CaasOperatorAgent) ChangeConfig(mutate agent.ConfigMutator) error {
	err := op.AgentConf.ChangeConfig(mutate)
	op.configChangedVal.Set(true)
	return errors.Trace(err)
}
