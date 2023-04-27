package migraterepos_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/redhatinsights/edge-api/cmd/migraterepos/migraterepos"
	"github.com/redhatinsights/edge-api/cmd/migraterepos/repairrepos"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrate custom repositories", func() {

	Context("MigrateAllCustomRepositories", func() {
		// in this scenario all the repos urls does not exist in remote content-sources repositories
		var initialDefaultLimit int
		var initialConfAuth bool
		var orgID1 string
		var orgID2 string

		var repos []models.ThirdPartyRepo

		BeforeEach(func() {
			initialDefaultLimit = repairrepos.DefaultDataLimit
			repairrepos.DefaultDataLimit = 1
			initialConfAuth = config.Get().Auth
			// force auth, this should make the client to put the identity in the request headers
			config.Get().Auth = true

			// enable migration feature
			err := os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			// use org prefixes to preserve sorting
			orgID1 = "orgID1" + faker.UUIDHyphenated()
			orgID2 = "orgID2" + faker.UUIDHyphenated()
			repos = []models.ThirdPartyRepo{
				{OrgID: orgID1, Name: faker.Name(), URL: faker.URL()},
				{OrgID: orgID1, Name: faker.Name(), URL: faker.URL()},
				{OrgID: orgID2, Name: faker.Name(), URL: faker.URL()},
				{OrgID: orgID2, Name: faker.Name(), URL: faker.URL()},
			}
			err = db.DB.Create(&repos).Error
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			config.Get().Auth = initialConfAuth
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
			repairrepos.DefaultDataLimit = initialDefaultLimit
		})

		It("migrate all repos successfully", func() {
			contentSourcesGetCallIndex := 0
			contentSourcesPostCallIndex := 0
			ts := httptest.NewServer(dependencies.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rhIndent, err := common.GetIdentityInstanceFromContext(r.Context())
				Expect(err).ToNot(HaveOccurred())
				w.Header().Set("Content-Type", "application/json")
				var repo models.ThirdPartyRepo
				switch r.Method {
				case http.MethodGet:
					// for any GET request return an empty response , that should be interpreted as
					// "repo with url" or "repo with name" does not exist
					// this should be called twice the number of repos
					// one time for each repo with GET repo by url
					// one time for each repo with GET repo by name
					repoIndex := int32(contentSourcesGetCallIndex / 2)
					repo = repos[repoIndex]
					urlQueryValues := r.URL.Query()

					if (contentSourcesGetCallIndex+1)%2 == 1 {
						// this is the repo first GET call, the url query one
						url := urlQueryValues.Get("url")
						Expect(url).To(Equal(repo.URL))

					} else {
						// this is the repo second GET call, the name query one
						name := urlQueryValues.Get("name")
						Expect(name).To(Equal(repo.Name))
					}
					w.WriteHeader(http.StatusOK)
					err := json.NewEncoder(w).Encode(&repositories.ListRepositoriesResponse{})
					Expect(err).ToNot(HaveOccurred())
					contentSourcesGetCallIndex++
				case http.MethodPost:
					// this should be one time call for each repo
					w.WriteHeader(http.StatusCreated)
					repo = repos[contentSourcesPostCallIndex]
					var reqRepository repositories.Repository
					body, err := io.ReadAll(r.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(body).ToNot(BeNil())
					err = json.Unmarshal(body, &reqRepository)
					Expect(err).ToNot(HaveOccurred())

					Expect(reqRepository.Name).To(Equal(repo.Name))
					Expect(reqRepository.URL).To(Equal(repo.URL))

					// ensure the identity orgID is the same as the currently processed repo
					Expect(repo.OrgID).To(Equal(rhIndent.Identity.OrgID))

					// set a new uuid
					reqRepository.UUID = uuid.New()
					// set orgID from identity (note the org is not sent in the post body request payload)
					reqRepository.OrgID = rhIndent.Identity.OrgID
					// return the updated content-sources repo
					err = json.NewEncoder(w).Encode(&reqRepository)
					Expect(err).ToNot(HaveOccurred())
					contentSourcesPostCallIndex++
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			})))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			err := migraterepos.MigrateAllCustomRepositories()
			Expect(err).ToNot(HaveOccurred())

			Expect(contentSourcesGetCallIndex).To(Equal(2 * len(repos)))
			Expect(contentSourcesPostCallIndex).To(Equal(len(repos)))

			var dbRepos []models.ThirdPartyRepo
			err = db.DB.Where("org_id IN (?)", []string{orgID1, orgID2}).Order("org_id ASC, id ASC").Find(&dbRepos).Error
			Expect(err).ToNot(HaveOccurred())

			Expect(len(dbRepos)).To(Equal(len(repos)))
			for ind, repo := range dbRepos {
				initialRepo := repos[ind]
				Expect(repo.Name).To(Equal(initialRepo.Name))
				Expect(repo.OrgID).To(Equal(initialRepo.OrgID))
				Expect(repo.URL).To(Equal(initialRepo.URL))
				// the initial uuid was empty
				Expect(initialRepo.UUID).To(BeEmpty())
				// after repo migration to content-sources repository an uuid is assigned to local db repo
				Expect(repo.UUID).ToNot(BeEmpty())
				_, err := uuid.Parse(repo.UUID)
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

	Context("MigrateAllCustomRepositories with existing content-sources repo url", func() {
		// in this scenario a repo with url exists in content sources repository
		var initialDefaultLimit int
		var orgID string
		var url string

		var repo models.ThirdPartyRepo
		var contentSourcesRepo repositories.Repository

		BeforeEach(func() {
			initialDefaultLimit = repairrepos.DefaultDataLimit
			repairrepos.DefaultDataLimit = 1

			// enable migration feature
			err := os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			// use org prefixes to preserve sorting
			orgID = faker.UUIDHyphenated()
			url = faker.URL()
			repo = models.ThirdPartyRepo{OrgID: orgID, Name: faker.Name(), URL: url}
			err = db.DB.Create(&repo).Error
			Expect(err).ToNot(HaveOccurred())
			contentSourcesRepo = repositories.Repository{
				Name:             faker.Name(),
				URL:              url,
				OrgID:            orgID,
				UUID:             uuid.New(),
				GpgKey:           faker.UUIDHyphenated(),
				DistributionArch: "x86_64",
				PackageCount:     12,
			}
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
			repairrepos.DefaultDataLimit = initialDefaultLimit
		})

		It("repository migrated successfully", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					urlQueryValues := r.URL.Query()
					// in this scenario we send only one request to content-sources, the url one
					url := urlQueryValues.Get("url")
					Expect(url).To(Equal(repo.URL))

					w.WriteHeader(http.StatusOK)
					// sent the contentSourcesRepo as the requested existing content-sources repo
					err := json.NewEncoder(w).Encode(&repositories.ListRepositoriesResponse{Data: []repositories.Repository{contentSourcesRepo}})
					Expect(err).ToNot(HaveOccurred())
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			err := migraterepos.MigrateAllCustomRepositories()
			Expect(err).ToNot(HaveOccurred())

			// ensure local repo has been updated with remote content-sources repository uuid, and other data
			var dbRepo models.ThirdPartyRepo
			err = db.DB.First(&dbRepo, repo.ID).Error
			Expect(err).ToNot(HaveOccurred())

			Expect(dbRepo.UUID).To(Equal(contentSourcesRepo.UUID.String()))
			Expect(dbRepo.GpgKey).To(Equal(contentSourcesRepo.GpgKey))
			Expect(dbRepo.DistributionArch).To(Equal(contentSourcesRepo.DistributionArch))
			Expect(dbRepo.PackageCount).To(Equal(contentSourcesRepo.PackageCount))
		})
	})

	Context("MigrateAllCustomRepositories with existing content-sources repo name", func() {
		// in this scenario a repo with same name exists in content sources repository
		// a new name should be generated with original repo name as prefix
		var initialDefaultLimit int
		var orgID string
		var repoName string
		var newUUID uuid.UUID

		var repo models.ThirdPartyRepo

		BeforeEach(func() {
			initialDefaultLimit = repairrepos.DefaultDataLimit
			repairrepos.DefaultDataLimit = 1
			newUUID = uuid.New()

			// enable migration feature
			err := os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			// use org prefixes to preserve sorting
			orgID = faker.UUIDHyphenated()
			repoName = faker.Name()
			repo = models.ThirdPartyRepo{OrgID: orgID, Name: repoName, URL: faker.URL()}
			err = db.DB.Create(&repo).Error
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
			repairrepos.DefaultDataLimit = initialDefaultLimit
		})

		It("repository migrated successfully", func() {
			contentSourcesGetCallIndex := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					switch contentSourcesGetCallIndex {
					case 0:
						// for url call should return an empty list as content-source repo with url has not to exist
						urlQueryValues := r.URL.Query()
						url := urlQueryValues.Get("url")
						Expect(url).To(Equal(repo.URL))

						w.WriteHeader(http.StatusOK)
						err := json.NewEncoder(w).Encode(&repositories.ListRepositoriesResponse{Data: []repositories.Repository{}})
						Expect(err).ToNot(HaveOccurred())
					case 1:
						// for name call should return an arbitrary repo to show that content-source repo with name exists
						urlQueryValues := r.URL.Query()
						name := urlQueryValues.Get("name")
						Expect(name).To(Equal(repoName))

						w.WriteHeader(http.StatusOK)
						err := json.NewEncoder(w).Encode(&repositories.ListRepositoriesResponse{
							Data: []repositories.Repository{{
								Name:  repoName,
								URL:   faker.URL(),
								OrgID: orgID,
							}},
						})
						Expect(err).ToNot(HaveOccurred())
					default:
						w.WriteHeader(http.StatusBadRequest)
					}
					contentSourcesGetCallIndex++
				case http.MethodPost:
					w.WriteHeader(http.StatusCreated)
					var reqRepository repositories.Repository
					body, err := io.ReadAll(r.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(body).ToNot(BeNil())
					err = json.Unmarshal(body, &reqRepository)
					Expect(err).ToNot(HaveOccurred())
					// expect that the repo name looks like repoName_uuid
					Expect(reqRepository.Name).ToNot(BeEmpty())
					repoNameSlice := strings.Split(reqRepository.Name, "_")
					Expect(len(repoNameSlice)).To(Equal(2))
					Expect(repoNameSlice[0]).To(Equal(repoName))
					_, err = uuid.Parse(repoNameSlice[1])
					Expect(err).ToNot(HaveOccurred())
					// set a uuid
					reqRepository.UUID = newUUID
					err = json.NewEncoder(w).Encode(&reqRepository)
					Expect(err).ToNot(HaveOccurred())
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			err := migraterepos.MigrateAllCustomRepositories()
			Expect(err).ToNot(HaveOccurred())

			// ensure local repo has been updated with remote content-sources repository uuid
			var dbRepo models.ThirdPartyRepo
			err = db.DB.First(&dbRepo, repo.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(dbRepo.UUID).To(Equal(newUUID.String()))
		})
	})

	Context("MigrateAllCustomRepositories: return error on client failure", func() {
		var orgID string
		var repo models.ThirdPartyRepo

		BeforeEach(func() {
			// enable migration feature
			err := os.Setenv(feature.MigrateCustomRepositories.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())
			orgID = faker.UUIDHyphenated()
			repo = models.ThirdPartyRepo{OrgID: orgID, Name: faker.Name(), URL: faker.URL()}
			err = db.DB.Create(&repo).Error
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when client fail", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer ts.Close()
			config.Get().ContentSourcesURL = ts.URL

			err := migraterepos.MigrateAllCustomRepositories()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(repositories.ErrRepositoryRequestResponse))
		})
	})

	Context("migration feature disabled", func() {

		BeforeEach(func() {
			// ensure migration feature is disabled, feature should be disabled by default
			err := os.Unsetenv(feature.MigrateCustomRepositories.EnvVar)
			Expect(err).ToNot(HaveOccurred())
		})

		It("repair urls should not be available", func() {
			err := migraterepos.MigrateAllCustomRepositories()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(repairrepos.ErrMigrationNotAvailable))
		})
	})
})
