// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"fmt"
	"strings"
	"time"

	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/txn"

	"github.com/juju/juju/permission"
)

// modelUserLastConnectionDoc is updated by the apiserver whenever the user
// connects over the API. This update is not done using mgo.txn so the values
// could well change underneath a normal transaction and as such, it should
// NEVER appear in any transaction asserts. It is really informational only as
// far as everyone except the api server is concerned.
type modelUserLastConnectionDoc struct {
	ID             string    `bson:"_id"`
	ModelUUID      string    `bson:"model-uuid"`
	UserName       string    `bson:"user"`
	LastConnection time.Time `bson:"last-connection"`
}

// setModelAccess changes the user's access permissions on the model.
func (st *State) setModelAccess(access permission.Access, userGlobalKey, modelUUID string) error {
	if err := permission.ValidateModelAccess(access); err != nil {
		return errors.Trace(err)
	}
	op := updatePermissionOp(modelKey(modelUUID), userGlobalKey, access)
	err := st.runTransactionFor(modelUUID, []txn.Op{op})
	if err == txn.ErrAborted {
		return errors.NotFoundf("existing permissions")
	}
	return errors.Trace(err)
}

// LastModelConnection returns when this User last connected through the API
// in UTC. The resulting time will be nil if the user has never logged in.
func (st *State) LastModelConnection(user names.UserTag) (time.Time, error) {
	lastConnections, closer := st.getRawCollection(modelUserLastConnectionC)
	defer closer()

	username := user.Id()
	var lastConn modelUserLastConnectionDoc
	err := lastConnections.FindId(st.docID(username)).Select(bson.D{{"last-connection", 1}}).One(&lastConn)
	if err != nil {
		if err == mgo.ErrNotFound {
			err = errors.Wrap(err, NeverConnectedError(username))
		}
		return time.Time{}, errors.Trace(err)
	}

	return lastConn.LastConnection.UTC(), nil
}

// NeverConnectedError is used to indicate that a user has never connected to
// an model.
type NeverConnectedError string

// Error returns the error string for a user who has never connected to an
// model.
func (e NeverConnectedError) Error() string {
	return `never connected: "` + string(e) + `"`
}

// IsNeverConnectedError returns true if err is of type NeverConnectedError.
func IsNeverConnectedError(err error) bool {
	_, ok := errors.Cause(err).(NeverConnectedError)
	return ok
}

// UpdateLastModelConnection updates the last connection time of the model user.
func (st *State) UpdateLastModelConnection(user names.UserTag) error {
	return st.updateLastModelConnection(user, st.NowToTheSecond())
}

func (st *State) updateLastModelConnection(user names.UserTag, when time.Time) error {
	lastConnections, closer := st.getCollection(modelUserLastConnectionC)
	defer closer()

	lastConnectionsW := lastConnections.Writeable()

	// Update the safe mode of the underlying session to not require
	// write majority, nor sync to disk.
	session := lastConnectionsW.Underlying().Database.Session
	session.SetSafe(&mgo.Safe{})

	lastConn := modelUserLastConnectionDoc{
		ID:             st.docID(strings.ToLower(user.Id())),
		ModelUUID:      st.ModelUUID(),
		UserName:       user.Id(),
		LastConnection: when,
	}
	_, err := lastConnectionsW.UpsertId(lastConn.ID, lastConn)
	return errors.Trace(err)
}

// ModelUser a model userAccessDoc.
func (st *State) modelUser(modelUUID string, user names.UserTag) (userAccessDoc, error) {
	modelUser := userAccessDoc{}
	modelUsers, closer := st.getCollectionFor(modelUUID, modelUsersC)
	defer closer()

	username := strings.ToLower(user.Id())
	err := modelUsers.FindId(username).One(&modelUser)
	if err == mgo.ErrNotFound {
		return userAccessDoc{}, errors.NotFoundf("model user %q", username)
	}
	if err != nil {
		return userAccessDoc{}, errors.Trace(err)
	}
	// DateCreated is inserted as UTC, but read out as local time. So we
	// convert it back to UTC here.
	modelUser.DateCreated = modelUser.DateCreated.UTC()
	return modelUser, nil
}

func createModelUserOps(modelUUID string, user, createdBy names.UserTag, displayName string, dateCreated time.Time, access permission.Access) []txn.Op {
	creatorname := createdBy.Id()
	doc := &userAccessDoc{
		ID:          userAccessID(user),
		ObjectUUID:  modelUUID,
		UserName:    user.Id(),
		DisplayName: displayName,
		CreatedBy:   creatorname,
		DateCreated: dateCreated,
	}

	ops := []txn.Op{
		createPermissionOp(modelKey(modelUUID), userGlobalKey(userAccessID(user)), access),
		{
			C:      modelUsersC,
			Id:     userAccessID(user),
			Assert: txn.DocMissing,
			Insert: doc,
		},
	}
	return ops
}

func removeModelUserOps(modelUUID string, user names.UserTag) []txn.Op {
	return []txn.Op{
		removePermissionOp(modelKey(modelUUID), userGlobalKey(userAccessID(user))),
		{
			C:      modelUsersC,
			Id:     userAccessID(user),
			Assert: txn.DocExists,
			Remove: true,
		}}
}

// removeModelUser removes a user from the database.
func (st *State) removeModelUser(user names.UserTag) error {
	ops := removeModelUserOps(st.ModelUUID(), user)
	err := st.runTransaction(ops)
	if err == txn.ErrAborted {
		err = errors.NewNotFound(nil, fmt.Sprintf("model user %q does not exist", user.Id()))
	}
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

type UserModelInfo interface {
	Name() string
	UUID() string
	Type() string
	Owner() names.UserTag
}

// UserModel contains information about an model that a
// user has access to.
type UserModel struct {
	iaasModel *Model
	caasModel *CAASModel

	model UserModelInfo

	User names.UserTag
}

func (um *UserModel) Name() string {
	return um.model.Name()
}

func (um *UserModel) UUID() string {
	return um.model.UUID()
}

func (um *UserModel) Type() string {
	return um.model.Type()
}

func (um *UserModel) Owner() names.UserTag {
	return um.model.Owner()
}

func (um *UserModel) CAASModel() *CAASModel {
	return um.caasModel
}

func (um *UserModel) IAASModel() *Model {
	return um.iaasModel
}

// LastConnection returns the last time the user has connected to the
// model.
func (um *UserModel) LastConnection() (time.Time, error) {
	// XXX implement for CAAS.
	if um.iaasModel == nil {
		return time.Time{}, errors.Trace(NeverConnectedError(um.User.Id()))
	}

	lastConnections, lastConnCloser := um.iaasModel.st.getRawCollection(modelUserLastConnectionC)
	defer lastConnCloser()

	lastConnDoc := modelUserLastConnectionDoc{}
	id := ensureModelUUID(um.iaasModel.ModelTag().Id(), strings.ToLower(um.User.Id()))
	err := lastConnections.FindId(id).Select(bson.D{{"last-connection", 1}}).One(&lastConnDoc)
	if (err != nil && err != mgo.ErrNotFound) || lastConnDoc.LastConnection.IsZero() {
		return time.Time{}, errors.Trace(NeverConnectedError(um.User.Id()))
	}

	return lastConnDoc.LastConnection, nil
}

// allUserModels returns a list of all models with the user they belong to.
func (st *State) allUserModels() ([]*UserModel, error) {
	var result []*UserModel

	iaasModels, err := st.AllModels()
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, model := range iaasModels {
		result = append(result, &UserModel{iaasModel: model, model: model, User: model.Owner()})
	}

	caasModels, err := st.AllCAASModels()
	if err != nil {
		return nil, errors.Trace(err)
	}
	for _, model := range caasModels {
		result = append(result, &UserModel{caasModel: model, model: model, User: model.Owner()})
	}

	return result, nil
}

// ModelsForUser returns a list of models that the user
// is able to access.
func (st *State) ModelsForUser(user names.UserTag) ([]*UserModel, error) {
	// Consider the controller permissions overriding Model permission, for
	// this case the only relevant one is superuser.
	// The mgo query below wont work for superuser case because it needs at
	// least one model user per model.
	access, err := st.UserAccess(user, st.controllerTag)
	if err == nil && access.Access == permission.SuperuserAccess {
		return st.allUserModels()
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, errors.Trace(err)
	}

	// XXX CAAS

	// Since there are no groups at this stage, the simplest way to get all
	// the models that a particular user can see is to look through the
	// model user collection. A raw collection is required to support
	// queries across multiple models.
	modelUsers, userCloser := st.getRawCollection(modelUsersC)
	defer userCloser()

	var userSlice []userAccessDoc
	err = modelUsers.Find(bson.D{{"user", user.Id()}}).Select(bson.D{{"object-uuid", 1}, {"_id", 1}}).All(&userSlice)
	if err != nil {
		return nil, err
	}

	var result []*UserModel
	for _, doc := range userSlice {
		modelTag := names.NewModelTag(doc.ObjectUUID)
		model, err := st.GetModel(modelTag)
		if err != nil {
			return nil, errors.Trace(err)
		}

		if model.Life() != Dead && model.MigrationMode() != MigrationModeImporting {
			result = append(result, &UserModel{iaasModel: model, model: model, User: user})
		}
	}

	return result, nil
}

// IsControllerAdmin returns true if the user specified has Super User Access.
func (st *State) IsControllerAdmin(user names.UserTag) (bool, error) {
	ua, err := st.UserAccess(user, st.ControllerTag())
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Trace(err)
	}
	return ua.Access == permission.SuperuserAccess, nil
}
