package models

import (
	"fmt"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
)

func TestStaticDeltaStateStruct(t *testing.T) {
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

	assert.Equal(t, state.Name, name, "Name is not equal to the expected value")
	assert.Equal(t, state.OrgID, orgID, "OrgID is not equal to the expected value")
	assert.Equal(t, state.Status, status, "Status is not equal to the expected value")
	assert.Equal(t, state.URL, url, "URL is not equal to the expected value")
}
