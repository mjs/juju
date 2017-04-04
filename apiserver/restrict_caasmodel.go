// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/juju/utils/set"
)

// caasModelFacadeNames are the root names that only apply to a CAAS
// model.
var caasModelFacadeNames = set.NewStrings(
	"CAASApplication",
)

func caasModelFacadesOnly(facadeName, _ string) error {
	if !isCAASModelFacade(facadeName) {
		return errors.NewNotSupported(nil, fmt.Sprintf("facade %q not supported for a CAAS model API connection", facadeName))
	}
	return nil
}

// isCAASModelFacade reports whether the given facade name can be accessed
// using the controller connection.
func isCAASModelFacade(facadeName string) bool {
	return caasModelFacadeNames.Contains(facadeName) || commonFacadeNames.Contains(facadeName)
}
