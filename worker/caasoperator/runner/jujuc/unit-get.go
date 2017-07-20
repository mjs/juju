// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/gnuflag"
)

// UnitGetCommand implements the unit-get command.
type UnitGetCommand struct {
	cmd.CommandBase
	ctx Context
	Key string
	out cmd.Output
}

func NewUnitGetCommand(ctx Context) (cmd.Command, error) {
	return &UnitGetCommand{ctx: ctx}, nil
}

func (c *UnitGetCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "unit-get",
		Args:    "<setting>",
		Purpose: "print public-address or private-address",
	}
}

func (c *UnitGetCommand) SetFlags(f *gnuflag.FlagSet) {
	c.out.AddFlags(f, "smart", cmd.DefaultFormatters)
}

func (c *UnitGetCommand) Init(args []string) error {
	if args == nil {
		return errors.New("no setting specified")
	}
	if args[0] != "private-address" && args[0] != "public-address" {
		return fmt.Errorf("unknown setting %q", args[0])
	}
	c.Key = args[0]
	return cmd.CheckEmpty(args[1:])
}

func (c *UnitGetCommand) Run(ctx *cmd.Context) error {
	if c.Key != "private-address" {
		return fmt.Errorf("in prototype, CAAS units only support unit-get private-address")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	podName := strings.Replace(c.ctx.UnitName(), "/", "-", 1)
	pod, err := client.CoreV1().Pods("default").Get(podName)
	if err != nil {
		return err
	}
	return c.out.Write(ctx, pod.Status.PodIP)
}
