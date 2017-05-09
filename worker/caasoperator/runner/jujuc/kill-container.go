// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/gnuflag"
)

// KillContainerCommand implements the run-container command.
type KillContainerCommand struct {
	cmd.CommandBase
	ctx  Context
	name string
}

// NewKillContainerCommand makes a jujuc run-container command.
func NewKillContainerCommand(ctx Context) (cmd.Command, error) {
	return &KillContainerCommand{ctx: ctx}, nil
}

func (c *KillContainerCommand) Info() *cmd.Info {
	doc := `
Asks the container runtime to kill a container with the specified
name.
`
	return &cmd.Info{
		Name:    "run-container",
		Args:    "<name>",
		Purpose: "tell container framework to kill one container",
		Doc:     doc,
	}
}

func (c *KillContainerCommand) SetFlags(f *gnuflag.FlagSet) {
}

func (c *KillContainerCommand) Init(args []string) error {
	if len(args) != 1 {
		return errors.Errorf("invalid args, requires <name>")
	}
	c.name = args[0]
	return nil
}

func (c *KillContainerCommand) Run(ctx *cmd.Context) error {
	return c.ctx.KillContainer(c.name)
}
