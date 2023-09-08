package models

import (
	"fmt"
	"testing"

	"github.com/bxcodec/faker/v3"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	FailMessageNotNil                    string = "Error is not nil"
	FailMessageInitialStatusDoesNotMatch string = "Initial static delta status does not match expected"
	FailMessageStateNotReady             string = "Static delta state is not READY"
	FailMessageIsReadyReturnedError      string = "Static delta IsReady returned an error"
)

// TestStaticDeltaStateDelete tests deletion of a static delta
func TestStaticDeltaStateDelete(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	url := faker.URL()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusReady

	// write a state entry into the DB
	initState := &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
		URL:    url,
	}
	err := initState.Save(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, FailMessageNotNil)
	assert.Equal(t, status, initState.Status, FailMessageInitialStatusDoesNotMatch)

	// Delete the static delta state
	testState := &StaticDeltaState{
		Name:  name,
		OrgID: orgID,
	}

	deleteErr := testState.Delete(log.NewEntry(log.StandardLogger()))
	assert.Nil(t, deleteErr, "Static delta Delete returned an error")

	dbState, err := initState.Query(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, StaticDeltaStatusNotFound, dbState.Status, "State status is not NOTFOUND")
	assert.Equal(t, nil, err, FailMessageNotNil)
}

// TestStaticDeltaStateExistsFalse is true if the static delta state DOES NOT EXIST
func TestStaticDeltaStateExistsFalse(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	url := faker.URL()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusGenerating

	state := &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
		URL:    url,
	}

	// test no record in the DB
	exists, err := state.Exists(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, FailMessageNotNil)
	assert.False(t, exists, "Static delta state record exists")
}

// TestStaticDeltaStateExistsTrue tests true if the static delta state EXISTS
func TestStaticDeltaStateExistsTrue(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusGenerating

	// write a state entry into the DB
	initState := &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
	}
	err := initState.Save(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, FailMessageNotNil)
	assert.Equal(t, status, initState.Status, FailMessageInitialStatusDoesNotMatch)

	// query IsReady status from a fresh state struct
	testState := &StaticDeltaState{
		Name:  name,
		OrgID: orgID,
	}

	exists, existsErr := testState.Exists(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, existsErr, FailMessageNotNil)
	assert.True(t, exists, "Static delta state record does not exist")
	assert.Equal(t, name, testState.Name, "Initial static delta name does not match expected name")
}

// TestStaticDeltaStateIsReadyFalse is true if the static delta state is not READY
func TestStaticDeltaStateIsReadyFalse(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	url := faker.URL()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusGenerating

	state := &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
		URL:    url,
	}

	// write a state entry into the DB
	err := state.Save(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, FailMessageNotNil)

	isReady, readyErr := state.IsReady(log.NewEntry(log.StandardLogger()))

	assert.False(t, isReady, FailMessageStateNotReady)
	assert.Nil(t, readyErr, FailMessageIsReadyReturnedError)
}

// TestStaticDeltaStateIsReadyTrue tests true if the static delta state is READY
func TestStaticDeltaStateIsReadyTrue(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusReady

	// write a state entry into the DB
	initState := &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
	}
	err := initState.Save(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, FailMessageNotNil)
	assert.Equal(t, status, initState.Status, FailMessageInitialStatusDoesNotMatch)

	// query IsReady status from a fresh state struct
	testState := &StaticDeltaState{
		Name:  name,
		OrgID: orgID,
	}

	isReady, readyErr := testState.IsReady(log.NewEntry(log.StandardLogger()))
	assert.True(t, isReady, FailMessageStateNotReady)
	assert.Nil(t, readyErr, FailMessageIsReadyReturnedError)
	assert.Equal(t, status, testState.Status, FailMessageInitialStatusDoesNotMatch)
}

// TestStaticDeltaStateQueryFound tests reading static delta state
func TestStaticDeltaStateQueryFound(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	url := faker.URL()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusGenerating

	state := &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
		URL:    url,
	}

	// write a state entry into the DB
	err := state.Save(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, FailMessageNotNil)

	// test with a state entry in the DB
	dbState, err := state.Query(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, name, dbState.Name, "State name does not match the expected name")
	assert.Equal(t, nil, err, FailMessageNotNil)
}

// TestStaticDeltaStateQueryNotFound tests reading static delta state for name not in DB
func TestStaticDeltaStateQueryNotFound(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)

	// test without a state entry in the DB
	state := &StaticDeltaState{
		Name:  name,
		OrgID: orgID,
	}

	dbState, err := state.Query(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, StaticDeltaStatusNotFound, dbState.Status, "State status is not NOTFOUND")
	assert.Equal(t, nil, err, FailMessageNotNil)
}
