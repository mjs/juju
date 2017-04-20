// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package params

type CAASApplicationsDeploy struct {
	Applications []CAASApplicationDeploy `json:"applications"`
}

type CAASApplicationDeploy struct {
	ApplicationName string `json:"application"`
	CharmURL        string `json:"charm-url"`
	Channel         string `json:"channel"`
	ConfigYAML      string `json:"config-yaml"` // Takes precedence over config if both are present.
	NumUnits        int    `json:"num-units"`
}
