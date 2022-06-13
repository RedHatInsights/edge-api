package services_test

import (
	"context"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"strconv"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
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

		It("Custom repo should not be created without account", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL()}
			_, err := customReposService.CreateThirdPartyRepo(&repo, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Account is not set"))
		})

		It("Custom repo should not be created with empty name", func() {
			account := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: "", URL: faker.URL()}
			_, err := customReposService.CreateThirdPartyRepo(&repo, account)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository name cannot be empty"))
		})

		It("Custom repo should not be created with empty URL", func() {
			account := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: ""}
			_, err := customReposService.CreateThirdPartyRepo(&repo, account)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository URL cannot be empty"))
		})

		It("Custom repo should be created successfully", func() {
			account := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL()}
			newRepo, err := customReposService.CreateThirdPartyRepo(&repo, account)
			Expect(err).ToNot(HaveOccurred())
			Expect(newRepo.Name).To(Equal(repo.Name))
			Expect(newRepo.URL).To(Equal(repo.URL))
			Expect(newRepo.Account).To(Equal(account))
		})

		It("Custom repo should not be created if name already exists", func() {
			account := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL()}
			_, err := customReposService.CreateThirdPartyRepo(&repo, account)
			Expect(err).ToNot(HaveOccurred())
			repo2 := models.ThirdPartyRepo{Name: repo.Name, URL: faker.URL()}
			_, err = customReposService.CreateThirdPartyRepo(&repo2, account)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository already exists"))
		})
	})
	Context("Custom repos creation with validation of URL", func() {
		It("Custom repo should not be created with invalid URL", func() {
			account := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: "http//google.com"}
			_, err := customReposService.CreateThirdPartyRepo(&repo, account)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid URL"))
		})

		It("Custom repo should be created with valid URL", func() {
			account := faker.UUIDHyphenated()
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: "https://google.com/"}
			_, err := customReposService.CreateThirdPartyRepo(&repo, account)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Custom repos update", func() {
		account := common.DefaultAccount
		repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), Account: account}
		result := db.DB.Create(&repo)
		repo2 := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), Account: account}
		result2 := db.DB.Create(&repo2)

		It("Custom repo should not be updated if name exists ", func() {
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result2.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{Name: repo2.Name}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, account, strconv.FormatUint(uint64(repo.ID), 10))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository already exists"))
		})

		It("Custom repo url should not be updated if image exists", func() {
			image := models.Image{
				Account:                account,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo2},
				Status:                 models.ImageStatusSuccess,
			}
			result := db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{URL: faker.URL()}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, account, strconv.FormatUint(uint64(repo2.ID), 10))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository is used by some images"))
		})

		It("Custom repo name should be updated successfully if image exists", func() {
			image := models.Image{
				Account:                account,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo2},
				Status:                 models.ImageStatusSuccess,
			}
			result := db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{Name: faker.URL()}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, account, strconv.FormatUint(uint64(repo2.ID), 10))
			Expect(err).ToNot(HaveOccurred())
		})

		It("Custom repo URL should be updated successfully even if image exists (error status)", func() {
			image := models.Image{
				Account:                account,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo2},
				Status:                 models.ImageStatusError,
			}
			result := db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			upRepo := models.ThirdPartyRepo{Name: faker.URL()}
			err := customReposService.UpdateThirdPartyRepo(&upRepo, account, strconv.FormatUint(uint64(repo2.ID), 10))
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("Custom repos delete", func() {
		account := common.DefaultAccount

		It("Custom repo should be deleted successfully", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), Account: account}
			result := db.DB.Create(&repo)
			Expect(result.Error).ToNot(HaveOccurred())
			deletedRepo, err := customReposService.DeleteThirdPartyRepoByID(strconv.FormatUint(uint64(repo.ID), 10))
			Expect(err).ToNot(HaveOccurred())
			Expect(deletedRepo.ID).To(Equal(repo.ID))
		})

		It("Custom repo should not be deleted when used by image", func() {
			repo := models.ThirdPartyRepo{Name: faker.UUIDHyphenated(), URL: faker.URL(), Account: account}
			result := db.DB.Create(&repo)
			Expect(result.Error).ToNot(HaveOccurred())
			image := models.Image{
				Account:                account,
				ThirdPartyRepositories: []models.ThirdPartyRepo{repo},
			}
			result = db.DB.Create(&image)
			Expect(result.Error).ToNot(HaveOccurred())
			_, err := customReposService.DeleteThirdPartyRepoByID(strconv.FormatUint(uint64(repo.ID), 10))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("custom repository is used by some images"))
		})
	})
})
