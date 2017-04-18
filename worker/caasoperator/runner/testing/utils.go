// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package testing

import (
	"path/filepath"
	"runtime"

	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/leadership"
)

type fops interface {
	// MkDir provides the functionality of gc.C.MkDir().
	MkDir() string
}

// RealPaths implements Paths for tests that do touch the filesystem.
type RealPaths struct {
	tools         string
	charm         string
	socket        string
	metricsspool  string
	componentDirs map[string]string
	fops          fops
}

func osDependentSockPath(c *gc.C) string {
	sockPath := filepath.Join(c.MkDir(), "test.sock")
	if runtime.GOOS == "windows" {
		return `\\.\pipe` + sockPath[2:]
	}
	return sockPath
}

func NewRealPaths(c *gc.C) RealPaths {
	return RealPaths{
		tools:         c.MkDir(),
		charm:         c.MkDir(),
		socket:        osDependentSockPath(c),
		metricsspool:  c.MkDir(),
		componentDirs: make(map[string]string),
		fops:          c,
	}
}

func (p RealPaths) GetMetricsSpoolDir() string {
	return p.metricsspool
}

func (p RealPaths) GetToolsDir() string {
	return p.tools
}

func (p RealPaths) GetCharmDir() string {
	return p.charm
}

func (p RealPaths) GetJujucSocket() string {
	return p.socket
}

func (p RealPaths) ComponentDir(name string) string {
	if dirname, ok := p.componentDirs[name]; ok {
		return dirname
	}
	p.componentDirs[name] = filepath.Join(p.fops.MkDir(), name)
	return p.componentDirs[name]
}

type FakeTracker struct {
	leadership.Tracker
}

func (FakeTracker) ApplicationName() string {
	return "application-name"
}
