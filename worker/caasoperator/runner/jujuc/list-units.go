// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"github.com/juju/cmd"
	"github.com/juju/gnuflag"
)

type ListUnitsCommand struct {
	cmd.CommandBase
	ctx Context
	out cmd.Output
}

func NewListUnitsCommand(ctx Context) (cmd.Command, error) {
	return &ListUnitsCommand{ctx: ctx}, nil
}

func (c *ListUnitsCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "list-units",
		Purpose: "print the units for an application",
		Doc:     "",
	}
}

func (c *ListUnitsCommand) SetFlags(f *gnuflag.FlagSet) {
	c.out.AddFlags(f, "smart", cmd.DefaultFormatters)
}

func (c *ListUnitsCommand) Init(args []string) error {
	return cmd.CheckEmpty(args)
}

func (c *ListUnitsCommand) Run(ctx *cmd.Context) error {
	units, err := c.ctx.AllCAASUnits()
	if err != nil {
		return err
	}
	out := make(map[string]string)
	for _, unit := range units {
		out[unit.Name()] = string(unit.Life())
	}
	c.out.Write(ctx, out)
	return nil
}
