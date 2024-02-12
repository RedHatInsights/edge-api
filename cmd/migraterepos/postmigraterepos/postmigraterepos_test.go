// FIXME: golangci-lint
// nolint:revive
package postmigraterepos

import (
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	"gorm.io/gorm"
)

var _ = Describe("Post migration delete custom repositories", func() {

	Context("Successfully delete one custom repo", func() {
		// in this scenario the repo that connect to image will not delete and the repo that isn't connect will delete,
		// both of repos already migrated to content source
		var orgID1 string
		var orgID2 string
		var image1 *models.Image

		var repos1 []models.ThirdPartyRepo
		var repos2 []models.ThirdPartyRepo

		BeforeEach(func() {
			// enable post migration delete custom repositories feature
			err := os.Setenv(feature.PostMigrateDeleteCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			// initialize only once for all the tests
			if orgID1 == "" && orgID2 == "" {
				// use org prefixes to preserve sorting
				orgID1 = "orgID1" + faker.UUIDHyphenated()
				orgID2 = "orgID2" + faker.UUIDHyphenated()
				repos1 = []models.ThirdPartyRepo{
					{OrgID: orgID1, Name: faker.Name(), URL: faker.URL(), UUID: faker.UUIDHyphenated()},
					{OrgID: orgID1, Name: faker.Name(), URL: faker.URL(), UUID: faker.UUIDHyphenated()},
				}
				err = db.DB.Create(&repos1).Error
				Expect(err).ToNot(HaveOccurred())
				repos2 = []models.ThirdPartyRepo{
					{OrgID: orgID1, Name: faker.Name(), URL: faker.URL(), UUID: faker.UUIDHyphenated()},
					{OrgID: orgID2, Name: faker.Name(), URL: faker.URL(), UUID: faker.UUIDHyphenated()},
				}
				err = db.DB.Create(&repos2).Error
				Expect(err).ToNot(HaveOccurred())
				image1 = &models.Image{
					OrgID:                  orgID1,
					Name:                   faker.UUIDHyphenated(),
					ThirdPartyRepositories: repos1,
				}
				err = db.DB.Create(&image1).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.PostMigrateDeleteCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
		})

		It("PostMigrateDeleteCustomRepo run successfully", func() {
			res, err := PostMigrateDeleteCustomRepo()
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(int64(len(repos2))))
		})

		It("successfully deleted the expected custom repositories", func() {
			for _, repoToBeDeleted := range repos2 {
				var repo models.ThirdPartyRepo
				err := db.DB.First(&repo, repoToBeDeleted.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			}
		})

		It("should not delete the custom repositories that are linked to images", func() {
			for _, repoNotDeleted := range repos1 {
				var repo models.ThirdPartyRepo
				err := db.DB.First(&repo, repoNotDeleted.ID).Error
				Expect(err).ToNot(HaveOccurred())
				Expect(repo.ID).To(Equal(repoNotDeleted.ID))
			}
		})
	})

	Context("post migration delete feature disabled", func() {

		BeforeEach(func() {
			// ensure migration feature is disabled, feature should be disabled by default
			err := os.Unsetenv(feature.PostMigrateDeleteCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
		})

		It("post migration delete should not work when the feature is disabled", func() {
			res, err := PostMigrateDeleteCustomRepo()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ErrPostMigrationNotAvailable))
			Expect(res).To(Equal(int64(0)))
		})
	})
	Context("post migration delete feature enabled but there is no repo to delete", func() {

		BeforeEach(func() {
			// ensure migration feature is disabled, feature should be disabled by default
			err := os.Setenv(feature.PostMigrateDeleteCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not return result if there is no repository to delete", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			res, err := PostMigrateDeleteCustomRepo()
			Expect(res).To(Equal(int64(0)))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
