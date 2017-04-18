// Copyright 2012-2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package runner

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/utils/clock"
	utilexec "github.com/juju/utils/exec"

	"github.com/juju/juju/worker/caasoperator/runner/context"
	"github.com/juju/juju/worker/caasoperator/runner/debug"
	"github.com/juju/juju/worker/caasoperator/runner/jujuc"
	jujuos "github.com/juju/utils/os"
)

var logger = loggo.GetLogger("juju.worker.caasoperator.runner")

// Runner is responsible for invoking commands in a context.
type Runner interface {

	// CAASContext returns the context against which the runner executes.
	CAASContext() CAASContext

	// RunHook executes the hook with the supplied name.
	RunHook(name string) error

	// RunAction executes the action with the supplied name.
	// RunAction(name string) error

	// RunCommands executes the supplied script.
	RunCommands(commands string) (*utilexec.ExecResponse, error)
}

// CAASContext exposes jujuc.Context, and additional methods needed by Runner.
type CAASContext interface {
	jujuc.Context
	Id() string
	HookVars(paths context.Paths) ([]string, error)
	SetProcess(process context.HookProcess)
	HasExecutionSetApplicationStatus() bool
	ResetExecutionSetApplicationStatus()
	Prepare() error
	Flush(badge string, failure error) error
}

// NewRunner returns a Runner backed by the supplied context and paths.
func NewRunner(context CAASContext, paths context.Paths) Runner {
	return &runner{context, paths}
}

// runner implements Runner.
type runner struct {
	context CAASContext
	paths   context.Paths
}

func (runner *runner) CAASContext() CAASContext {
	return runner.context
}

// RunCommands exists to satisfy the Runner interface.
func (runner *runner) RunCommands(commands string) (*utilexec.ExecResponse, error) {
	result, err := runner.runCommandsWithTimeout(commands, 0, clock.WallClock)
	return result, runner.context.Flush("run commands", err)
}

// runCommandsWithTimeout is a helper to abstract common code between run commands and
// juju-run as an action
func (runner *runner) runCommandsWithTimeout(commands string, timeout time.Duration, clock clock.Clock) (*utilexec.ExecResponse, error) {
	srv, err := runner.startJujucServer()
	if err != nil {
		return nil, err
	}
	defer srv.Close()

	env, err := runner.context.HookVars(runner.paths)
	if err != nil {
		return nil, errors.Trace(err)
	}
	command := utilexec.RunParams{
		Commands:    commands,
		WorkingDir:  runner.paths.GetCharmDir(),
		Environment: env,
		Clock:       clock,
	}

	err = command.Run()
	if err != nil {
		return nil, err
	}
	runner.context.SetProcess(hookProcess{command.Process()})

	var cancel chan struct{}
	if timeout != 0 {
		cancel = make(chan struct{})
		go func() {
			<-clock.After(timeout)
			close(cancel)
		}()
	}

	// Block and wait for process to finish
	return command.WaitWithCancel(cancel)
}

func encodeBytes(input []byte) (value string, encoding string) {
	if utf8.Valid(input) {
		value = string(input)
		encoding = "utf8"
	} else {
		value = base64.StdEncoding.EncodeToString(input)
		encoding = "base64"
	}
	return value, encoding
}

// RunAction exists to satisfy the Runner interface.
func (runner *runner) RunAction(actionName string) error {
	return runner.runCharmHookWithLocation(actionName, "actions")
}

// RunHook exists to satisfy the Runner interface.
func (runner *runner) RunHook(hookName string) error {
	return runner.runCharmHookWithLocation(hookName, "hooks")
}

func (runner *runner) runCharmHookWithLocation(hookName, charmLocation string) error {
	srv, err := runner.startJujucServer()
	if err != nil {
		return err
	}
	defer srv.Close()

	env, err := runner.context.HookVars(runner.paths)
	if err != nil {
		return errors.Trace(err)
	}
	if jujuos.HostOS() == jujuos.Windows {
		// TODO(fwereade): somehow consolidate with utils/exec?
		// We don't do this on the other code path, which uses exec.RunCommands,
		// because that already has handling for windows environment requirements.
		env = mergeWindowsEnvironment(env, os.Environ())
	}

	debugctx := debug.NewHooksContext(runner.context.ApplicationName())
	if session, _ := debugctx.FindSession(); session != nil && session.MatchHook(hookName) {
		logger.Infof("executing %s via debug-hooks", hookName)
		err = session.RunHook(hookName, runner.paths.GetCharmDir(), env)
	} else {
		err = runner.runCharmHook(hookName, env, charmLocation)
	}
	return runner.context.Flush(hookName, err)
}

func (runner *runner) runCharmHook(hookName string, env []string, charmLocation string) error {
	charmDir := runner.paths.GetCharmDir()
	hook, err := searchHook(charmDir, filepath.Join(charmLocation, hookName))
	if err != nil {
		return err
	}
	hookCmd := hookCommand(hook)
	ps := exec.Command(hookCmd[0], hookCmd[1:]...)
	ps.Env = env
	ps.Dir = charmDir
	outReader, outWriter, err := os.Pipe()
	if err != nil {
		return errors.Errorf("cannot make logging pipe: %v", err)
	}
	ps.Stdout = outWriter
	ps.Stderr = outWriter
	hookLogger := &hookLogger{
		r:      outReader,
		done:   make(chan struct{}),
		logger: runner.getLogger(hookName),
	}
	go hookLogger.run()
	err = ps.Start()
	outWriter.Close()
	if err == nil {
		// Record the *os.Process of the hook
		runner.context.SetProcess(hookProcess{ps.Process})
		// Block until execution finishes
		err = ps.Wait()
	}
	hookLogger.stop()
	return errors.Trace(err)
}

func (runner *runner) startJujucServer() (*jujuc.Server, error) {
	// Prepare server.
	getCmd := func(ctxId, cmdName string) (cmd.Command, error) {
		if ctxId != runner.context.Id() {
			return nil, errors.Errorf("expected context id %q, got %q", runner.context.Id(), ctxId)
		}
		return jujuc.NewCommand(runner.context, cmdName)
	}
	srv, err := jujuc.NewServer(getCmd, runner.paths.GetJujucSocket())
	if err != nil {
		return nil, err
	}
	go srv.Run()
	return srv, nil
}

func (runner *runner) getLogger(hookName string) loggo.Logger {
	return loggo.GetLogger(fmt.Sprintf("unit.%s.%s", runner.context.ApplicationName(), hookName))
}

type hookProcess struct {
	*os.Process
}

func (p hookProcess) Pid() int {
	return p.Process.Pid
}
