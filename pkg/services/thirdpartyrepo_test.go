// FIXME: golangci-lint
// nolint:govet,revive,typecheck
package services_test

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	log "github.com/osbuild/logging/pkg/logrus"
)

var _ = Describe("ThirdPartyRepos basic functions", func() {
	var (
		ctx                context.Context
		customReposService services.ThirdPartyRepoServiceInterface
	)
	BeforeEach(func() {
		ctx = context.Background()
		customReposService = services.NewThirdPartyRepoService(ctx, log.NewEntry(log.StandardLogger()))
	})

	Context("Custom repos creation", func() {

		It("Custom repo should not be created without an org_id", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL()}
			_, err := customReposService.CreateThirdPartyRepo(&repo, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Org ID is not set"))
		})

		It("Custom repo should not be created with empty name", func() {
			orgID := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: "", URL: faker.URL()}
			_, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository name cannot be empty"))
		})

		It("Custom repo should not be created with empty URL", func() {
			orgID := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: ""}
			_, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository URL cannot be empty"))
		})

		It("Custom repo should be created successfully", func() {
			orgID := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: models.AddSlashToURL(faker.URL())}
			newRepo, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).ToNot(HaveOccurred())
			Expect(newRepo.Name).To(Equal(repo.Name))
			Expect(newRepo.URL).To(Equal(repo.URL))
			Expect(newRepo.OrgID).To(Equal(orgID))
		})

		It("ThirdPartyRepoNameExists  return True when repository exists", func() {
			orgID := faker.UUIDHyphenated()
			name := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: name, URL: faker.URL(), OrgID: orgID}
			result := db.DB.Create(&repo)
			Expect(result.Error).ToNot(HaveOccurred())
			value, err := customReposService.ThirdPartyRepoNameExists(orgID, name)
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(BeTrue())
		})

		It("ThirdPartyRepoNameExists  return false when repository does not exists", func() {
			value, err := customReposService.ThirdPartyRepoNameExists(faker.UUIDHyphenated(), faker.UUIDHyphenated())
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(BeFalse())
		})

		It("ThirdPartyRepoNameExists return error when org_id is empty", func() {
			_, err := customReposService.ThirdPartyRepoNameExists("", faker.UUIDHyphenated())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Org ID is not set"))
		})

		It("ThirdPartyRepoNameExists return error when repository name is empty", func() {
			_, err := customReposService.ThirdPartyRepoNameExists(faker.UUIDHyphenated(), "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository name cannot be empty"))
		})

		It("Custom repo should not be created if name already exists", func() {
			orgID := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL()}
			_, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).ToNot(HaveOccurred())
			repo2 := models.ThirdPartyRepo{Name: repo.Name, URL: faker.URL()}
			_, err = customReposService.CreateThirdPartyRepo(&repo2, orgID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository already exists"))
		})
	})
	Context("Custom repos creation with validation of URL", func() {
		DescribeTable("Custom repos creation with invalid URL", func(url string) {
			orgID := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: url}
			_, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid URL"))
		},
			Entry("validate invalid URL with : missing", "http//google.com"),
			Entry("validate invalid URL with https:// missing", "google.com"),
			Entry("validate invalid URL without https:// and . missing", "foo/bar"),
			Entry("validate invalid URL with number", "5432/bar"),
			Entry("validate invalid URL with symbols", "http://valid-internet-host-com/llll"),
		)

		It("Custom repo should be created with valid URL", func() {
			orgID := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: "https://google.com/"}
			_, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Custom repos update", func() {
		orgID := common.DefaultOrgID
		repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), OrgID: orgID}
		result := db.DB.Create(&repo)
		repo2 := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), OrgID: orgID}
		result2 := db.DB.Create(&repo2)

		It("Custom repo should not be updated if name exists ", func() {
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result2.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{Name: repo2.Name}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, orgID, strconv.FormatUint(uint64(repo.ID), 10))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository already exists"))
		})

		It("Custom repo url should not be updated if image exists", func() {
			image := models.Image{
				OrgID:                  orgID,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo2},
				Status:                 models.ImageStatusSuccess,
			}
			result := db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{URL: faker.URL()}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, orgID, strconv.FormatUint(uint64(repo2.ID), 10))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository is used by some images"))
		})

		It("Custom repo name should be updated successfully if image exists", func() {
			image := models.Image{
				OrgID:                  orgID,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo2},
				Status:                 models.ImageStatusSuccess,
			}
			result := db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{Name: faker.URL()}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, orgID, strconv.FormatUint(uint64(repo2.ID), 10))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Custom repo URL should be updated successfully even if image exists (error status)", func() {
			image := models.Image{
				OrgID:                  orgID,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo2},
				Status:                 models.ImageStatusError,
			}
			result := db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{Name: faker.URL()}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, orgID, strconv.FormatUint(uint64(repo2.ID), 10))
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("Custom repos delete", func() {
		orgID := common.DefaultOrgID

		It("Custom repo should be deleted successfully", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), OrgID: orgID}
			result := db.DB.Create(&repo)
			Expect(result.Error).ToNot(HaveOccurred())
			deletedRepo, err := customReposService.DeleteThirdPartyRepoByID(strconv.FormatUint(uint64(repo.ID), 10))
			Expect(err).ToNot(HaveOccurred())
			Expect(deletedRepo.ID).To(Equal(repo.ID))
		})

		It("Custom repo should not be deleted when used by image", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), OrgID: orgID}
			result := db.DB.Create(&repo)
			Expect(result.Error).ToNot(HaveOccurred())
			image := models.Image{
				OrgID:                  orgID,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo},
			}
			result = db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			_, err := customReposService.DeleteThirdPartyRepoByID(strconv.FormatUint(uint64(repo.ID), 10))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository is used by some images"))
		})
	})

	Context("ThirdPartyRepoURLExists", func() {
		var orgID string
		var repo models.ThirdPartyRepo
		var url string
		var otherURL string
		BeforeEach(func() {
			orgID = common.DefaultOrgID
			// important the urls are without trailing slash "/"
			url = fmt.Sprintf("http://%s-example.com/repo", faker.UUIDHyphenated())
			otherURL = fmt.Sprintf("http://%s-example.com/repo", faker.UUIDHyphenated())
			repo = models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: url, OrgID: orgID}
			err := db.DB.Create(&repo).Error
			Expect(err).ToNot(HaveOccurred())
		})

		It("should find that the repo exists with initial url", func() {
			exists, err := customReposService.ThirdPartyRepoURLExists(orgID, url)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("should find that the repo exists with cleaned url", func() {
			exists, err := customReposService.ThirdPartyRepoURLExists(orgID, models.AddSlashToURL(url))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("should not find that the repo exists with otherURL", func() {
			exists, err := customReposService.ThirdPartyRepoURLExists(orgID, otherURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should not find that the repo exists with cleaned otherURL", func() {
			exists, err := customReposService.ThirdPartyRepoURLExists(orgID, models.AddSlashToURL(otherURL))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should return error when org is undefined", func() {
			_, err := customReposService.ThirdPartyRepoURLExists("", url)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(services.OrgIDNotSetMsg))
		})
		It("should return error when url is undefined", func() {
			_, err := customReposService.ThirdPartyRepoURLExists(orgID, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(services.ThirdPartyRepositoryURLIsEmptyMsg))
		})

		It("should not create a repo with url when a repo with that url exists", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: url}
			_, err := customReposService.CreateThirdPartyRepo(&repo, orgID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(services.ThirdPartyRepositoryWithURLAlreadyExistsMsg))
		})

		It("should not update a repo with url when a repo with that url exists", func() {
			existingURL := fmt.Sprintf("http://%s-example.com/repo", faker.UUIDHyphenated())
			existingRepo := models.ThirdPartyRepo{
				Name:  faker.UUIDHyphenated(),
				URL:   existingURL,
				OrgID: orgID,
			}
			err := db.DB.Create(&existingRepo).Error
			Expect(err).ToNot(HaveOccurred())
			updateRepo := models.ThirdPartyRepo{
				Name: faker.UUIDHyphenated(),
				URL:  existingURL,
			}
			err = customReposService.UpdateThirdPartyRepo(&updateRepo, orgID, strconv.Itoa(int(repo.ID)))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(services.ThirdPartyRepositoryWithURLAlreadyExistsMsg))
		})
	})
})
