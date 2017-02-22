// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/utils/clock"
	"gopkg.in/juju/names.v2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/txn"
)

type CAASState struct {
	modelTag      names.ModelTag
	controllerTag names.ControllerTag
	session       *mgo.Session
	database      Database
	clock         clock.Clock
}

func newCAASState(
	parentSt *State,
	modelTag names.ModelTag,
	clock clock.Clock,
) (_ *CAASState, err error) {
	session := parentSt.MongoSession().Copy()

	defer func() {
		if err != nil {
			session.Close()
		}
	}()

	rawDB := session.DB(jujuDB)
	database, err := allCollections().Load(rawDB, modelTag.Id(), nil)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &CAASState{
		modelTag:      modelTag,
		controllerTag: parentSt.ControllerTag(),
		session:       session,
		database:      database,
		clock:         clock,
	}, nil
}

func (st *CAASState) start() error {
	// XXX workers
	return nil
}

func (st *CAASState) Model() (*CAASModel, error) {
	return nil, nil // XXX
}

// runTransaction is a convenience method delegating to the state's Database.
// XXX this should be shared - need a base database/txn type that's shared by State and CAASState
func (st *CAASState) runTransaction(ops []txn.Op) error {
	runner, closer := st.database.TransactionRunner()
	defer closer()
	return runner.RunTransaction(ops)
}

func (st *CAASState) Close() error {
	// XXX child worker shutdown here
	st.session.Close()
	return nil
}

// modelSetupOps returns the transactions necessary to set up a CAAS model.
func (st *CAASState) modelSetupOps(modelUUID, controllerUUID string, args CAASModelArgs) ([]txn.Op, error) {
	/* XXX
	// Inc ref count for hosted models.
	if controllerModelUUID != modelUUID {
		ops = append(ops, incHostedModelCountOp())
	}
	*/
	return []txn.Op{
		createCAASModelOp(
			args.Owner, args.Name, modelUUID, controllerUUID,
			args.Endpoint, args.CertData, args.KeyData, args.CAData),
		createUniqueOwnerModelNameOp(args.Owner, args.Name),
	}, nil
}

// createCAASModelOp returns the operation needed to create
// a caasModel document with the given name and UUID.
func createCAASModelOp(
	owner names.UserTag,
	name, uuid, controllerUUID string,
	endpoint string,
	certData, keyData, caData []byte,
) txn.Op {
	doc := &caasModelDoc{
		UUID:           uuid,
		Name:           name,
		Life:           Alive,
		Owner:          owner.Id(),
		ControllerUUID: controllerUUID,
		Endpoint:       endpoint,
		CertData:       certData,
		KeyData:        keyData,
		CAData:         caData,
	}
	return txn.Op{
		C:      caasModelsC,
		Id:     uuid,
		Assert: txn.DocMissing,
		Insert: doc,
	}
}
