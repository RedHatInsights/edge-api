package services_test

import (
	"context"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
)

var _ = Describe("Repo Service Test", func() {
	var ctrl *gomock.Controller
	var testRepo models.Repo
	var repoService services.RepoServiceInterface
	ctx := context.Background()
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		repoService = services.NewRepoService(ctx, log.NewEntry(log.StandardLogger()))

	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("test repo id", func() {
		When("is valid", func() {
			It("repo retrieved successfully", func() {
				testRepo = models.Repo{
					URL:    "www.test.com",
					Status: models.RepoStatusSuccess,
				}
				result := db.DB.Create(&testRepo)
				Expect(result.Error).ToNot(HaveOccurred())
				repo, err := repoService.GetRepoByID(&testRepo.ID)
				Expect(err).To(BeNil())
				Expect(repo.Status).To(Equal(testRepo.Status))
				Expect(repo.Model.ID).To(Equal(testRepo.Model.ID))
			})
		})
		When("is not valid with nil value", func() {
			It("repo id doesnt create and verify that we dont get repo id with nil vale", func() {
				var id1 *uint
				repo, err := repoService.GetRepoByID(id1)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("image repository is undefined"))
				Expect(repo).To(BeNil())
			})
		})
		When("is not valid with repo id that doesnt exist", func() {
			It("repo id doesnt create and verify that repo id doesnt exist in db", func() {
				id := faker.ID
				uid := uint(id[0])
				repo, err := repoService.GetRepoByID(&uid)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("record not found"))
				Expect(repo).To(BeNil())
			})
		})
	})
})
