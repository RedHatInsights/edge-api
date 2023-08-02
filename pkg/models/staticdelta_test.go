package models

import (
	"fmt"
	"testing"

	"github.com/bxcodec/faker/v3"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestStaticDeltaStateReadFromStore tests reading static delta state
func TestStaticDeltaStateStore(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	url := faker.URL()
	fromCommit := faker.UUIDDigit()
	toCommit := faker.UUIDDigit()
	name := fmt.Sprintf("%s-%s", fromCommit, toCommit)
	status := StaticDeltaStatusGenerating

	// test without a state entry in the DB
	state := &StaticDeltaState{}

	dbState, err := state.ReadFromStore(log.NewEntry(log.StandardLogger()), orgID, name)
	assert.Equal(t, "", dbState.Name, "State name does not match the expected name")
	assert.Equal(t, nil, err, "Error is not nil")

	state = &StaticDeltaState{
		Name:   name,
		OrgID:  orgID,
		Status: status,
		URL:    url,
	}

	// write a state entry into the DB
	err = state.SaveToStore(log.NewEntry(log.StandardLogger()))
	assert.Equal(t, nil, err, "Error is not nil")

	// test with a state entry in the DB
	dbState, err = state.ReadFromStore(log.NewEntry(log.StandardLogger()), orgID, name)
	assert.Equal(t, name, dbState.Name, "State name does not match the expected name")
	assert.Equal(t, nil, err, "Error is not nil")
}
