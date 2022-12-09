package models

import (
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestCommitsBeforeCreate(t *testing.T) {
	cases := []struct {
		Name     string
		Input    Commit
		Expected error
	}{
		{
			"Missing orgID",
			Commit{},
			ErrOrgIDIsMandatory,
		},
		{
			"Can be created",
			Commit{OrgID: faker.UUIDHyphenated()},
			nil,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Input.BeforeCreate(db.DB)
			assert.Equal(t, test.Expected, got)
		})
	}
}
