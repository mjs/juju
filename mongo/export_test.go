// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package mongo

import (
	"github.com/juju/juju/service"
	"github.com/juju/juju/service/common"
)

var (
	MakeJournalDirs = makeJournalDirs
	MongoConfigPath = &mongoConfigPath
	NoauthCommand   = noauthCommand
	ProcessSignal   = &processSignal

	SharedSecretPath = sharedSecretPath
	SSLKeyPath       = sslKeyPath

	NewConf = newConf

	HostWordSize   = &hostWordSize
	RuntimeGOOS    = &runtimeGOOS
	AvailSpace     = &availSpace
	MinOplogSizeMB = &minOplogSizeMB
	MaxOplogSizeMB = &maxOplogSizeMB
	PreallocFile   = &preallocFile

	DefaultOplogSize  = defaultOplogSize
	FsAvailSpace      = fsAvailSpace
	PreallocFileSizes = preallocFileSizes
	PreallocFiles     = preallocFiles
)

func PatchService(patchValue func(interface{}, interface{}), data *service.FakeServiceData) {
	patchValue(&discoverService, func(name string) (mongoService, error) {
		svc := service.NewFakeService(name, common.Conf{})
		svc.FakeServiceData = data
		return svc, nil
	})
	patchValue(&newService, func(name string, conf common.Conf) (mongoService, error) {
		svc := service.NewFakeService(name, conf)
		svc.FakeServiceData = data
		return svc, nil
	})
}
