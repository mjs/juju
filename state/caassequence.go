// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"fmt"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// sequence safely increments a database backed sequence, returning
// the next value.
func (st *CAASState) sequence(name string) (int, error) {
	sequences, closer := st.database.GetCollection(sequenceC)
	defer closer()
	query := sequences.FindId(name)
	inc := mgo.Change{
		Update: bson.M{
			"$set": bson.M{
				"name":       name,
				"model-uuid": st.ModelUUID(),
			},
			"$inc": bson.M{"counter": 1},
		},
		Upsert: true,
	}
	result := &sequenceDoc{}
	_, err := query.Apply(inc, result)
	if err != nil {
		return -1, fmt.Errorf("cannot increment %q sequence number: %v", name, err)
	}
	return result.Counter, nil
}

// sequenceWithMin safely increments a database backed sequence,
// allowing for a minimum value for the sequence to be specified. The
// minimum value is used as an initial value for the first use of a
// particular sequence. The minimum value will also cause a sequence
// value to jump ahead if the minimum is provided that is higher than
// the current sequence value.
//
// The data manipulated by `sequence` and `sequenceWithMin` is the
// same. It is safe to mix the 2 methods for the same sequence.
//
// `sequence` is more efficient than `sequenceWithMin` and should be
// preferred if there is no minimum value requirement.
func (st *CAASState) sequenceWithMin(name string, minVal int) (int, error) {
	sequences, closer := st.getRawCollection(sequenceC)
	defer closer()
	updater := newDbSeqUpdater(sequences, st.ModelUUID(), name)
	return updateSeqWithMin(updater, minVal)
}
