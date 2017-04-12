// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/utils"
	"github.com/juju/utils/clock"
	"gopkg.in/juju/charm.v6-unstable"
	csparams "gopkg.in/juju/charmrepo.v2-unstable/csclient/params"
	"gopkg.in/juju/names.v2"
	"gopkg.in/juju/worker.v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/txn"

	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/state/storage"
	"github.com/juju/juju/state/watcher"
	"github.com/juju/juju/status"
)

type CAASState struct {
	modelTag      names.ModelTag
	controllerTag names.ControllerTag
	session       *mgo.Session
	database      Database
	workers       *caasWorkers
	clock         clock.Clock
}

func newCAASState(parentSt *State, modelTag names.ModelTag, clock clock.Clock) (_ *CAASState, err error) {
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

func (st *CAASState) modelClock() clock.Clock {
	return st.clock
}

func (st *CAASState) db() Database {
	return st.database
}

func (st *CAASState) txnLogWatcher() *watcher.Watcher {
	panic("not implemented")
}

func (st *CAASState) newStorage() storage.Storage {
	return storage.NewStorage(st.modelTag.Id(), st.session)
}

func (st *CAASState) start() error {
	ws, err := newCAASWorkers(st)
	if err != nil {
		return errors.Trace(err)
	}
	st.workers = ws
	return nil
}

func (st *CAASState) uuid() string {
	return st.ModelUUID()
}

// XXX extract to dbIdMapper and embedded into State and CAAS state
func (st *CAASState) docID(localID string) string {
	return ensureModelUUID(st.ModelUUID(), localID)
}

// XXX
func (st *CAASState) localID(ID string) string {
	modelUUID, localID, ok := splitDocID(ID)
	if !ok || modelUUID != st.ModelUUID() {
		return ID
	}
	return localID
}

// XXX
func (st *CAASState) strictLocalID(ID string) (string, error) {
	modelUUID, localID, ok := splitDocID(ID)
	if !ok || modelUUID != st.ModelUUID() {
		return "", errors.Errorf("unexpected id: %#v", ID)
	}
	return localID, nil
}

func (st *CAASState) ControllerUUID() string {
	return st.controllerTag.Id()
}

func (st *CAASState) ModelUUID() string {
	return st.modelTag.Id()
}

func (st *CAASState) CAASModel() (*CAASModel, error) {
	models, closer := st.database.GetCollection(caasModelsC)
	defer closer()

	model := &CAASModel{st: st}
	if err := model.refresh(models.FindId(st.ModelUUID())); err != nil {
		return nil, errors.Trace(err)
	}
	return model, nil
}

func (st *CAASState) ModelTag() names.ModelTag {
	return st.modelTag
}

func (st *CAASState) MongoSession() *mgo.Session {
	return st.session
}

func (st *CAASState) Close() error {
	if st.workers != nil {
		if err := worker.Stop(st.workers); err != nil {
			return errors.Annotatef(err, "failed to stop workers")
		}
	}
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

type AddCAASApplicationArgs struct {
	Name     string
	Charm    *Charm
	Channel  csparams.Channel
	Settings charm.Settings
	NumUnits int
}

// AddCAASApplication creates a new CAAS application, running the
// supplied charm, with the supplied name (which must be unique).
func (st *CAASState) AddCAASApplication(args AddCAASApplicationArgs) (_ *CAASApplication, err error) {
	defer errors.DeferredAnnotatef(&err, "cannot add application %q", args.Name)
	// Sanity checks.
	if !names.IsValidApplication(args.Name) {
		return nil, errors.Errorf("invalid name")
	}
	if args.Charm == nil {
		return nil, errors.Errorf("charm is nil")
	}

	if err := validateCharmVersion(args.Charm); err != nil {
		return nil, errors.Trace(err)
	}

	if exists, err := isNotDead(st, caasApplicationsC, args.Name); err != nil {
		return nil, errors.Trace(err)
	} else if exists {
		return nil, errors.Errorf("application already exists")
	}
	/* XXX
	if err := checkModelActive(st); err != nil {
		return nil, errors.Trace(err)
	}
	*/

	// The doc defaults to CharmModifiedVersion = 0, which is correct, since it
	// has, by definition, at its initial state.
	appDoc := &caasApplicationDoc{
		DocID:        args.Name,
		Name:         args.Name,
		ModelUUID:    st.ModelUUID(),
		CharmURL:     args.Charm.URL(),
		Channel:      string(args.Channel),
		Life:         Alive,
		PasswordHash: utils.AgentPasswordHash("password"), // XXX just for the prototype!
	}

	app := newCAASApplication(st, appDoc)

	statusDoc := statusDoc{
		ModelUUID:  st.ModelUUID(),
		Status:     status.Waiting,
		StatusInfo: status.MessageWaitForMachine, // XXX caas todo - review messages
		Updated:    st.clock.Now().UnixNano(),
		NeverSet:   true,
	}

	buildTxn := func(attempt int) ([]txn.Op, error) {
		// If we've tried once already and failed, check that
		// model may have been destroyed.
		if attempt > 0 {
			/* XXX
			if err := checkModelActive(st); err != nil {
				return nil, errors.Trace(err)
			}
			*/
			// Ensure a local application with the same name doesn't exist.
			if exists, err := isNotDead(st, caasApplicationsC, args.Name); err != nil {
				return nil, errors.Trace(err)
			} else if exists {
				return nil, errLocalApplicationExists
			}
		}
		// The addCAASApplicationOps does not include the model alive assertion,
		// so we add it here.
		ops := []txn.Op{
			assertCAASModelActiveOp(st.ModelUUID()),
		}
		addOps, err := addCAASApplicationOps(st, addCAASApplicationOpsArgs{
			appDoc:   appDoc,
			settings: map[string]interface{}(args.Settings),
		})
		if err != nil {
			return nil, errors.Trace(err)
		}
		ops = append(ops, addOps...)

		// Collect unit-adding operations.
		for x := 0; x < args.NumUnits; x++ {
			_, unitOps, err := app.addApplicationUnitOps(caasApplicationAddCAASUnitOpsArgs{})
			if err != nil {
				return nil, errors.Trace(err)
			}
			ops = append(ops, unitOps...)
		}

		return ops, nil
	}

	// At the last moment before inserting the application, prime status history.
	probablyUpdateStatusHistory(st, app.globalKey(), statusDoc)

	if err := st.db().Run(buildTxn); err != nil {
		return nil, errors.Trace(err)
	}

	// Refresh to pick the txn-revno.
	if err := app.Refresh(); err != nil {
		return nil, errors.Trace(err)
	}
	return app, nil
}

// ModelConfig returns the complete config for the model represented
// by this state.
func (st *CAASState) ModelConfig() (*config.Config, error) {
	modelSettings, err := readSettings(st, settingsC, modelGlobalKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return config.New(config.NoDefaults, modelSettings.Map())
}

func (st *CAASState) UpdateUploadedCharm(info CharmInfo) (*Charm, error) {
	return updateUploadedCharm(st, info)
}

func (st *CAASState) PrepareLocalCharmUpload(curl *charm.URL) (chosenURL *charm.URL, err error) {
	return prepareLocalCharmUpload(st, curl)
}

func (st *CAASState) PrepareStoreCharmUpload(curl *charm.URL) (*Charm, error) {
	return prepareStoreCharmUpload(st, curl)
}

// AllCharms returns all charms in state.
func (st *CAASState) AllCharms() ([]*Charm, error) {
	return allCharms(st)
}

// Charm returns the charm with the given URL. Charms pending upload
// to storage and placeholders are never returned.
func (st *CAASState) Charm(curl *charm.URL) (*Charm, error) {
	return loadCharm(st, curl)
}

// CAASApplication returns a caasapplication state by name.
func (st *CAASState) CAASApplication(name string) (_ *CAASApplication, err error) {
	applications, closer := st.db().GetCollection(caasApplicationsC)
	defer closer()

	if !names.IsValidApplication(name) {
		return nil, errors.Errorf("%q is not a valid application name", name)
	}
	sdoc := &caasApplicationDoc{}
	err = applications.FindId(name).One(sdoc)
	if err == mgo.ErrNotFound {
		return nil, errors.NotFoundf("application %q", name)
	}
	if err != nil {
		return nil, errors.Annotatef(err, "cannot get application %q", name)
	}
	return newCAASApplication(st, sdoc), nil
}

// CAASUnit returns a unit by name.
func (st *CAASState) CAASUnit(name string) (*CAASUnit, error) {
	if !names.IsValidUnit(name) {
		return nil, errors.Errorf("%q is not a valid unit name", name)
	}
	caasUnits, closer := st.db().GetCollection(caasUnitsC)
	defer closer()

	doc := caasUnitDoc{}
	err := caasUnits.FindId(name).One(&doc)
	if err == mgo.ErrNotFound {
		return nil, errors.NotFoundf("unit %q", name)
	}
	if err != nil {
		return nil, errors.Annotatef(err, "cannot get unit %q", name)
	}
	return newCAASUnit(st, &doc), nil
}
