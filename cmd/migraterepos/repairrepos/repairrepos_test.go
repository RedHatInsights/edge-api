package repairrepos_test

import (
	"os"
	"strings"

	"github.com/redhatinsights/edge-api/cmd/migraterepos/repairrepos"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repair custom repositories", func() {

	Context("RepairUrls", func() {
		var initialRepos []models.ThirdPartyRepo
		var reposLen = 10
		var badSuffix = "/without-slash  \t "
		var cleanedSuffix = "/without-slash/"
		var initialDefaultLimit int
		BeforeEach(func() {
			// disable the cleanup function to allow adding urls without slashes by returning the same initial url.
			models.RepoURLCleanUp = func(url string) string { return url }
			initialDefaultLimit = repairrepos.DefaultDataLimit
			repairrepos.DefaultDataLimit = 1

			for i := 0; i < reposLen; i++ {
				initialRepos = append(initialRepos, models.ThirdPartyRepo{OrgID: faker.UUIDHyphenated(), Name: "test-repair-urls" + faker.UUIDHyphenated(), URL: faker.URL() + badSuffix})
			}
			Expect(len(initialRepos)).To(Equal(reposLen))
			err := db.DB.Create(&initialRepos).Error
			Expect(err).ToNot(HaveOccurred())
			// enable migration feature
			err = os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			models.RepoURLCleanUp = models.AddSlashToURL
			// disable migration feature
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
			repairrepos.DefaultDataLimit = initialDefaultLimit
		})

		It("all initialRepos url should have been cleaned a slash added", func() {
			var repos []models.ThirdPartyRepo
			// ensure that all the urls are not clean
			err := db.DB.Where("name LIKE ?", "test-repair-urls%").Find(&repos).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repos)).To(Equal(reposLen))
			for _, repo := range repos {
				Expect(strings.HasSuffix(repo.URL, cleanedSuffix)).To(BeFalse())
			}

			affected, err := repairrepos.RepairUrls()
			Expect(err).ToNot(HaveOccurred())
			Expect(affected).To(Equal(affected))

			// ensure that all the urls are clean and that the repos base url has not changed
			err = db.DB.Where("name LIKE ?", "test-repair-urls%").Order("created_at ASC").Find(&repos).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(repos)).To(Equal(reposLen))

			for ind, repo := range repos {
				// ensure repo url is clean
				Expect(strings.HasSuffix(repo.URL, cleanedSuffix)).To(BeTrue())
				// ensure that base url has not changed
				Expect(strings.TrimRight(initialRepos[ind].URL, badSuffix)).To(Equal(strings.TrimRight(repo.URL, cleanedSuffix)))
			}
		})
	})

	Context("repairDuplicateImagesReposURLS", func() {
		var repoURL1 string
		var repoURL2 string
		var repoURL3 string
		var repoURL4 string
		var orgID string
		var image1 *models.Image
		var image2 *models.Image
		var initialDefaultLimit int
		initialDefaultLimit = repairrepos.DefaultDataLimit
		repairrepos.DefaultDataLimit = 1

		BeforeEach(func() {
			initialDefaultLimit = repairrepos.DefaultDataLimit
			repairrepos.DefaultDataLimit = 1
			// enable migration feature
			err := os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			// create two images with duplicates repos urls
			// create image1 and image2 only once
			if image1 == nil && image2 == nil {
				orgID = faker.UUIDHyphenated()
				repoURL1 = models.AddSlashToURL(faker.URL() + "/repo/")
				repoURL2 = models.AddSlashToURL(faker.URL() + "/repo/")
				repoURL3 = models.AddSlashToURL(faker.URL() + "/repo/")
				repoURL4 = models.AddSlashToURL(faker.URL() + "/repo/")
				image1 = &models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					ThirdPartyRepositories: []models.ThirdPartyRepo{
						// repoURL1 is duplicated two times
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL1,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL1,
						},
						// repoURL3 is duplicated two times
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL3,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL3,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL4,
						},
					},
				}
				err = db.DB.Create(&image1).Error
				Expect(err).ToNot(HaveOccurred())

				image2 = &models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					ThirdPartyRepositories: []models.ThirdPartyRepo{
						// repoURL2 is duplicated 4 times
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL2,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL2,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL2,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL2,
						},
						{
							OrgID: orgID,
							Name:  faker.UUIDHyphenated(),
							URL:   repoURL3,
						},
					},
				}
				err = db.DB.Create(&image2).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
			repairrepos.DefaultDataLimit = initialDefaultLimit
		})

		It("before repair image1 has duplicates", func() {
			var dbImage1 models.Image
			err := db.DB.Preload("ThirdPartyRepositories").First(&dbImage1, image1.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dbImage1.ThirdPartyRepositories)).To(Equal(5))
			var countOfRepoURL1 int
			var countOfRepoURL2 int
			var countOfRepoURL3 int
			var countOfRepoURL4 int
			for _, repo := range dbImage1.ThirdPartyRepositories {
				if repo.URL == repoURL1 {
					countOfRepoURL1++
				}
				if repo.URL == repoURL2 {
					countOfRepoURL2++
				}
				if repo.URL == repoURL3 {
					countOfRepoURL3++
				}
				if repo.URL == repoURL4 {
					countOfRepoURL4++
				}
			}
			Expect(countOfRepoURL1).To(Equal(2))
			// image1 has no repoURL2 url
			Expect(countOfRepoURL2).To(Equal(0))
			Expect(countOfRepoURL3).To(Equal(2))
			Expect(countOfRepoURL4).To(Equal(1))
		})

		It("before repair image2 has duplicates", func() {
			var dbImage2 models.Image
			err := db.DB.Preload("ThirdPartyRepositories").First(&dbImage2, image2.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dbImage2.ThirdPartyRepositories)).To(Equal(5))
			var countOfRepoURL1 int
			var countOfRepoURL2 int
			var countOfRepoURL3 int
			var countOfRepoURL4 int
			for _, repo := range dbImage2.ThirdPartyRepositories {
				if repo.URL == repoURL1 {
					countOfRepoURL1++
				}
				if repo.URL == repoURL2 {
					countOfRepoURL2++
				}
				if repo.URL == repoURL3 {
					countOfRepoURL3++
				}
				if repo.URL == repoURL4 {
					countOfRepoURL4++
				}
			}
			// image2 has no repoURL1 url
			Expect(countOfRepoURL1).To(Equal(0))
			Expect(countOfRepoURL2).To(Equal(4))
			Expect(countOfRepoURL3).To(Equal(1))
			// image2 has no repoURL4 url
			Expect(countOfRepoURL4).To(Equal(0))
		})

		It("RepairDuplicateImagesReposURLS should run successfully ", func() {
			err := repairrepos.RepairDuplicateImagesReposURLS()
			Expect(err).ToNot(HaveOccurred())
		})

		It("after repair should remove duplicate repos urls from image1", func() {
			var dbImage1 models.Image
			err := db.DB.Preload("ThirdPartyRepositories").First(&dbImage1, image1.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dbImage1.ThirdPartyRepositories)).To(Equal(3))
			var countOfRepoURL1 int
			var countOfRepoURL2 int
			var countOfRepoURL3 int
			var countOfRepoURL4 int
			for _, repo := range dbImage1.ThirdPartyRepositories {
				if repo.URL == repoURL1 {
					countOfRepoURL1++
				}
				if repo.URL == repoURL2 {
					countOfRepoURL2++
				}
				if repo.URL == repoURL3 {
					countOfRepoURL3++
				}
				if repo.URL == repoURL4 {
					countOfRepoURL4++
				}
			}
			Expect(countOfRepoURL1).To(Equal(1))
			// image1 has no repoURL2 url
			Expect(countOfRepoURL2).To(Equal(0))
			Expect(countOfRepoURL3).To(Equal(1))
			Expect(countOfRepoURL4).To(Equal(1))
		})

		It("after repair should remove duplicate repos urls from image2", func() {
			var dbImage2 models.Image
			err := db.DB.Preload("ThirdPartyRepositories").First(&dbImage2, image2.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(dbImage2.ThirdPartyRepositories)).To(Equal(2))
			var countOfRepoURL1 int
			var countOfRepoURL2 int
			var countOfRepoURL3 int
			var countOfRepoURL4 int
			for _, repo := range dbImage2.ThirdPartyRepositories {
				if repo.URL == repoURL1 {
					countOfRepoURL1++
				}
				if repo.URL == repoURL2 {
					countOfRepoURL2++
				}
				if repo.URL == repoURL3 {
					countOfRepoURL3++
				}
				if repo.URL == repoURL4 {
					countOfRepoURL4++
				}
			}
			// image2 has no repoURL1 url
			Expect(countOfRepoURL1).To(Equal(0))
			Expect(countOfRepoURL2).To(Equal(1))
			Expect(countOfRepoURL3).To(Equal(1))
			// image2 has no repoURL4 url
			Expect(countOfRepoURL4).To(Equal(0))
		})
	})

	Context("RepairDuplicates", func() {
		var repoURL1 string
		var repoURL2 string
		var repoURL3 string
		var orgID1 string
		var orgID2 string
		var repos []models.ThirdPartyRepo
		var image models.Image
		var orgPrefix string
		var initialDefaultLimit int

		BeforeEach(func() {
			// enable migration feature
			err := os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			initialDefaultLimit = repairrepos.DefaultDataLimit
			repairrepos.DefaultDataLimit = 1
			Expect(err).ToNot(HaveOccurred())
			if repos == nil {
				// initialize only once for all the tests
				repoURL1 = faker.URL() + "/repo/"
				repoURL2 = faker.URL() + "/repo/"
				repoURL3 = faker.URL() + "/repo/"
				orgPrefix = "test-duplicates-"

				orgID1 = orgPrefix + faker.UUIDHyphenated()
				orgID2 = orgPrefix + faker.UUIDHyphenated()

				repos = []models.ThirdPartyRepo{
					// repoURL1 is duplicated 2 times, and added to image
					{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: repoURL1},
					// this is the one that will be chosen to keep
					{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: repoURL1},
					// repoURL2 is duplicated 2 times
					{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: repoURL2},
					// this is the one that will be chosen to keep
					// it's also added to image
					{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: repoURL2},
					// other unique urls in this Org
					{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: faker.URL() + "/repo/"},
					{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: faker.URL() + "/repo/"},

					// repoURL1 is duplicated 2 times
					// repoURL1 is the same value as for orgID1
					{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: repoURL1},
					// this is the one that will be chosen to keep
					{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: repoURL1},
					// repoURL3 is duplicated 2 times
					{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: repoURL3},
					// this is the one that will be chosen to keep
					{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: repoURL3},
					// other unique urls in this Org
					{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: faker.URL() + "/repo/"},
					{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: faker.URL() + "/repo/"},
				}
				err := db.DB.Create(&repos).Error
				Expect(err).ToNot(HaveOccurred())
				// create an image with orgID1 , to have two repos that have duplicates
				image = models.Image{
					OrgID:                  orgID1,
					Name:                   faker.Name(),
					ThirdPartyRepositories: []models.ThirdPartyRepo{repos[0], repos[3]},
				}

				err = db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			// disable migration feature
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
			repairrepos.DefaultDataLimit = initialDefaultLimit
		})

		It("should remove all orgID1 repos with duplicate urls as expected", func() {
			err := repairrepos.RepairDuplicates()
			Expect(err).ToNot(HaveOccurred())

			var uniqueRepos []models.ThirdPartyRepo
			err = db.Org(orgID1, "").Order("id ASC").Find(&uniqueRepos).Error
			Expect(err).ToNot(HaveOccurred())

			Expect(len(uniqueRepos)).To(Equal(4))

			Expect(uniqueRepos[0].URL).To(Equal(repoURL1))
			Expect(uniqueRepos[1].URL).To(Equal(repoURL2))
			Expect(uniqueRepos[2].URL).To(Equal(repos[4].URL))
			Expect(uniqueRepos[3].URL).To(Equal(repos[5].URL))
		})

		It("should remove all orgID2 repos with duplicate urls as expected", func() {
			err := repairrepos.RepairDuplicates()
			Expect(err).ToNot(HaveOccurred())

			var uniqueRepos []models.ThirdPartyRepo
			err = db.Org(orgID2, "").Order("id ASC").Find(&uniqueRepos).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(len(uniqueRepos)).To(Equal(4))

			Expect(uniqueRepos[0].URL).To(Equal(repoURL1))
			Expect(uniqueRepos[1].URL).To(Equal(repoURL3))
			Expect(uniqueRepos[2].URL).To(Equal(repos[10].URL))
			Expect(uniqueRepos[3].URL).To(Equal(repos[11].URL))
		})

		It("should cleanup images from duplicates urls", func() {
			// refresh image data from db
			err := db.DB.Preload("ThirdPartyRepositories").First(&image, image.ID).Error
			Expect(err).ToNot(HaveOccurred())

			// ensure the same number of repos
			Expect(len(image.ThirdPartyRepositories)).To(Equal(2))

			// expect the repos urls has not been changed
			Expect(image.ThirdPartyRepositories[0].URL).To(Equal(repoURL1))
			Expect(image.ThirdPartyRepositories[1].URL).To(Equal(repoURL2))

			// ensure the right repo was kept in images
			Expect(image.ThirdPartyRepositories[0].ID).To(Equal(repos[1].ID))
			Expect(image.ThirdPartyRepositories[1].ID).To(Equal(repos[3].ID))
		})
	})

	Context("feature disabled", func() {

		BeforeEach(func() {
			// ensure migration feature is disabled, feature should be disabled by default
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
		})

		It("repair urls should not be available", func() {
			_, err := repairrepos.RepairUrls()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(repairrepos.ErrMigrationNotAvailable))
		})
		It("repair images repos urls should not be available", func() {
			err := repairrepos.RepairDuplicateImagesReposURLS()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(repairrepos.ErrMigrationNotAvailable))
		})

		It("repair duplicates should not be available", func() {
			_, err := repairrepos.RepairUrls()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(repairrepos.ErrMigrationNotAvailable))
		})
	})
})
