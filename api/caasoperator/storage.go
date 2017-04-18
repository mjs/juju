// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperator

import (
	"github.com/juju/juju/api/base"
)

type StorageAccessor struct {
	facade base.FacadeCaller
}
