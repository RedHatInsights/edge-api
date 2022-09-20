// FIXME: golangci-lint
// nolint:revive
package models

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("test base model", func() {
	Context("marshal base model", func() {
		baseModel := Model{
			ID:        1,
			CreatedAt: EdgeAPITime{Time: time.Now(), Valid: true},
			UpdatedAt: EdgeAPITime{Time: time.Time{}, Valid: false},
		}
		It("should marshal base model", func() {
			jsonBytes, err := json.Marshal(baseModel)
			Expect(err).To(BeNil())
			Expect(string(jsonBytes)).To(Equal(`{"ID":1,"CreatedAt":"` + baseModel.CreatedAt.Time.Format(time.RFC3339Nano) + `","UpdatedAt":null,"DeletedAt":null}`))
		})
		It("should unmarshal base model", func() {
			var baseModel2 Model
			jsonBytes := []byte(`{"ID":1,"CreatedAt":"` + baseModel.CreatedAt.Time.Format(time.RFC3339Nano) + `","UpdatedAt":null,"DeletedAt":null}`)
			err := json.Unmarshal(jsonBytes, &baseModel2)
			Expect(err).To(BeNil())
			Expect(baseModel2.ID).To(Equal(uint(1)))
			Expect(baseModel2.CreatedAt.Time.Format(time.RFC3339Nano)).To(Equal(baseModel.CreatedAt.Time.Format(time.RFC3339Nano)))
			Expect(baseModel2.UpdatedAt.Valid).To(BeFalse())
		})
	})
})
