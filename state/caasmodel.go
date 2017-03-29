// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/juju/mongo"
	"gopkg.in/juju/names.v2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/txn"
)

type CAASModel struct {
	st  *CAASState
	doc caasModelDoc
}

// XXX what we probably need is a base model collection with very
// simple docs and then separate caasModel and model collections which
// hold the bits specific to each.
type caasModelDoc struct {
	UUID string `bson:"_id"`
	Name string
	// XXX also add container runtime type
	Life           Life
	Owner          string `bson:"owner"`
	ControllerUUID string `bson:"controller-uuid"`
	Endpoint       string `bson:"endpoint"`
	// XXX these should live in a general store for easy
	// updating. Generalise the cloud credentials store.
	CertData []byte `bson:"cert-data"`
	KeyData  []byte `bson:"key-data"`
	CAData   []byte `bson:"ca-data"`
}

// CAASModelArgs is a params struct for creating a new CAAS model.
type CAASModelArgs struct {
	// XXX type (e.g. k8s, dc/os)
	UUID     string
	Name     string
	Owner    names.UserTag
	Endpoint string
	CertData []byte
	KeyData  []byte
	CAData   []byte
}

func (st *State) NewCAASModel(args CAASModelArgs) (*CAASModel, *CAASState, error) {
	caasSt, err := newCAASState(st, names.NewModelTag(args.UUID), st.clock)
	if err != nil {
		return nil, nil, errors.Annotate(err, "could not create state for new caas model")
	}
	defer func() {
		if err != nil {
			caasSt.Close()
		}
	}()

	modelOps, err := caasSt.modelSetupOps(args.UUID, st.controllerTag.Id(), args)
	if err != nil {
		return nil, nil, errors.Annotate(err, "failed to create new caas model")
	}

	err = caasSt.db().RunTransaction(modelOps)
	if err == txn.ErrAborted {
		// XXX extract
		// We have a  unique key restriction on the "owner" and "name" fields,
		// which will cause the insert to fail if there is another record with
		// the same "owner" and "name" in the collection. If the txn is
		// aborted, check if it is due to the unique key restriction.
		name := args.Name
		models, closer := st.db().GetCollection(caasModelsC)
		defer closer()
		envCount, countErr := models.Find(bson.D{
			{"owner", args.Owner.Id()},
			{"name", name}},
		).Count()
		if countErr != nil {
			err = errors.Trace(countErr)
		} else if envCount > 0 {
			err = errors.AlreadyExistsf("model %q for %s", name, args.Owner.Id())
		} else {
			err = errors.New("model already exists")
		}
	}
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	err = caasSt.start()
	if err != nil {
		return nil, nil, errors.Annotate(err, "could not start state for new caas model")
	}

	caasModel, err := caasSt.Model()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	return caasModel, caasSt, nil
}

func (st *State) IsCAASModel(uuid string) (bool, error) {
	models, closer := st.db().GetCollection(caasModelsC)
	defer closer()
	n, err := models.FindId(uuid).Count()
	if err != nil {
		return false, errors.Trace(err)
	}
	return n > 0, nil
}

func (m *CAASModel) UUID() string {
	return m.doc.UUID
}

func (m *CAASModel) Refresh() error {
	models, closer := m.st.db().GetCollection(caasModelsC)
	defer closer()
	return m.refresh(models.FindId(m.UUID()))
}

func (m *CAASModel) refresh(query mongo.Query) error {
	err := query.One(&m.doc)
	if err == mgo.ErrNotFound {
		return errors.NotFoundf("caas model")
	}
	return err
}
