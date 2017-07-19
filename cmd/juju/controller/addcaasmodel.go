// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package controller

import (
	"strings"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/modelmanager"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/cmd/juju/common"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
)

// NewAddCAASModelCommand returns a command to add a CAAS model.
func NewAddCAASModelCommand() cmd.Command {
	return modelcmd.WrapController(&addCAASModelCommand{
		newAPI: func(caller base.APICallCloser) AddCAASModelAPI {
			return modelmanager.NewClient(caller)
		},
	})
}

// addCAASModelCommand calls the API to add a new model.
type addCAASModelCommand struct {
	modelcmd.ControllerCommandBase
	apiRoot api.Connection
	newAPI  func(base.APICallCloser) AddCAASModelAPI

	Name           string
	KubeConfigPath string
}

const addCAASModelHelpDoc = `
XXX
`

func (c *addCAASModelCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "add-caas-model",
		Args:    "<model name> <kube-config-path>",
		Purpose: "Registers a container runtime instance with the controller.",
		Doc:     strings.TrimSpace(addCAASModelHelpDoc),
	}
}

func (c *addCAASModelCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ControllerCommandBase.SetFlags(f)
}

func (c *addCAASModelCommand) Init(args []string) error {
	if len(args) < 1 {
		return errors.New("model name is required")
	}
	if len(args) < 2 {
		return errors.New("path to kubernetes client config file is required")
	}
	c.Name, c.KubeConfigPath, args = args[0], args[1], args[2:]

	if !names.IsValidModelName(c.Name) {
		return errors.Errorf("%q is not a valid name: model names may only contain lowercase letters, digits and hyphens", c.Name)
	}
	return cmd.CheckEmpty(args)
}

type AddCAASModelAPI interface {
	CreateCAASModel(
		name, owner, endpoint string,
		caData, certData, keyData []byte,
		username, password string,
	) (string, error)
}

func (c *addCAASModelCommand) newAPIRoot() (api.Connection, error) {
	if c.apiRoot != nil {
		return c.apiRoot, nil
	}
	return c.NewAPIRoot()
}

func (c *addCAASModelCommand) Run(ctx *cmd.Context) error {
	api, err := c.newAPIRoot()
	if err != nil {
		return errors.Annotate(err, "opening API connection")
	}
	defer api.Close()

	store := c.ClientStore()
	controllerName, err := c.ControllerName()
	if err != nil {
		return errors.Trace(err)
	}
	accountDetails, err := store.AccountDetails(controllerName)
	if err != nil {
		return errors.Trace(err)
	}
	modelOwner := accountDetails.User

	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigPath)
	if err != nil {
		return errors.Annotate(err, "failed to read kubernetes config")
	}

	client := c.newAPI(api)
	modelUUID, err := client.CreateCAASModel(
		c.Name,
		modelOwner,
		config.Host,
		config.CAData,
		config.CertData,
		config.KeyData,
		config.Username,
		config.Password,
	)
	if err != nil {
		if params.IsCodeUnauthorized(err) {
			common.PermissionsMessage(ctx.Stderr, "add a model")
		}
		return errors.Trace(err)
	}

	if err := store.UpdateModel(controllerName, c.Name, jujuclient.ModelDetails{
		ModelUUID: modelUUID,
		Type:      jujuclient.CAASModel,
	}); err != nil {
		return errors.Trace(err)
	}
	if err := store.SetCurrentModel(controllerName, c.Name); err != nil {
		return errors.Trace(err)
	}

	ctx.Infof("Added '%s' CAAS model", c.Name)
	return nil
}
