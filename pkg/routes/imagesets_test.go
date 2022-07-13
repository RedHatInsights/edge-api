package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bxcodec/faker/v3"
	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"

	"github.com/golang/mock/gomock"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

func TestListAllImageSets(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req = req.WithContext(dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{}))
	handler := http.HandlerFunc(ListAllImageSets)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf("failed reading response body: %s", err.Error())
	}
	var result common.EdgeAPIPaginatedResponse
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		t.Errorf("failed decoding response body: %s", err.Error())
	}

	if result.Count != 0 && result.Data != "{}" {
		t.Errorf("handler returned wrong body: got %v, want %v",
			result.Count, 0)
	}
}

func TestGetImageSetByID(t *testing.T) {
	imageSetID := &models.ImageSet{}
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(req.Context(), imageSetKey, imageSetID)
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	ctx = dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{})
	req = req.WithContext(ctx)
	handler := http.HandlerFunc(GetImageSetsByID)

	handler.ServeHTTP(rr, req.WithContext(ctx))
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v",
			status, http.StatusOK)

	}
}

func TestSearchParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "bad status name",
			params: "name=image1&status=test",
			expectedError: []validationError{
				{Key: "status", Reason: "test is not a valid status. Status must be CREATED or BUILDING or ERROR or SUCCESS"},
			},
		},
		{
			name:   "bad sort_by",
			params: "sort_by=test",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must created_at or updated_at or name"},
			},
		},
		{
			name:   "bad sort_by and status",
			params: "sort_by=host&status=ONHOLD",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "host is not a valid sort_by. Sort-by must created_at or updated_at or name"},
				{Key: "status", Reason: "ONHOLD is not a valid status. Status must be CREATED or BUILDING or ERROR or SUCCESS"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		validateFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

func TestDetailSearchParams(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "bad status name",
			params: "name=image1&status=test",
			expectedError: []validationError{
				{Key: "status", Reason: "test is not a valid status. Status must be CREATED or BUILDING or ERROR or SUCCESS"},
			},
		},
		{
			name:   "bad sort_by",
			params: "sort_by=test",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must created_at or updated_at or name"},
			},
		},
		{
			name:   "bad sort_by and status",
			params: "sort_by=host&status=ONHOLD",
			expectedError: []validationError{
				{Key: "sort_by", Reason: "host is not a valid sort_by. Sort-by must created_at or updated_at or name"},
				{Key: "status", Reason: "ONHOLD is not a valid status. Status must be CREATED or BUILDING or ERROR or SUCCESS"},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets/1?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		validateFilterParams(next).ServeHTTP(w, req)

		resp := w.Result()
		jsonBody := []validationError{}
		err = json.NewDecoder(resp.Body).Decode(&jsonBody)
		if err != nil {
			t.Errorf("failed decoding response body: %s", err.Error())
		}
		for _, exErr := range te.expectedError {
			found := false
			for _, jsErr := range jsonBody {
				if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody)
			}
		}
	}
}

func TestImageSetFiltersParams(t *testing.T) {
	tt := []struct {
		name   string
		params string
	}{
		{
			name:   "sort by image_set name",
			params: "sort_by=-name",
		},
	}
	var sortTable = regexp.MustCompile(`image_sets`)
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets/1?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		got := imageSetFilters(req, db.DB.Model(&models.ImageSet{}))
		c := fmt.Sprintf("%v", got.Statement.Clauses["ORDER BY"].Expression)
		sortTable.MatchString(c)
		if !sortTable.MatchString(c) {
			t.Errorf("Expected ImageSet got: %v", c)
		}

	}
}

func TestImageSetDetailFiltersParams(t *testing.T) {
	tt := []struct {
		name   string
		params string
	}{
		{
			name:   "sort by image_set name",
			params: "sort_by=-name",
		},
	}
	var sortTable = regexp.MustCompile(`images`)
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets/1?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		got := imageDetailFilters(req, db.DB.Model(&models.ImageSet{}))
		c := fmt.Sprintf("%v", got.Statement.Clauses["ORDER BY"].Expression)
		sortTable.MatchString(c)
		if !sortTable.MatchString(c) {
			t.Errorf("Expected ImageSet got: %v", c)
		}

	}
}

var _ = Describe("ImageSets Route Test", func() {

	Context("Filters", func() {
		BeforeEach(func() {
			imageSet1 := &models.ImageSet{
				Name:  "image-set-1",
				OrgID: common.DefaultOrgID,
			}
			imageSet2 := &models.ImageSet{
				Name:  "image-set-2",
				OrgID: common.DefaultOrgID,
			}
			db.DB.Create(&imageSet1)
			db.DB.Create(&imageSet2)

			imageSuccess := models.Image{
				Name:       "image-success",
				ImageSetID: &imageSet1.ID,
				OrgID:      common.DefaultOrgID,
				Status:     models.ImageStatusSuccess,
			}
			imageError := models.Image{
				Name:       "image-error",
				ImageSetID: &imageSet2.ID,
				OrgID:      common.DefaultOrgID,
				Status:     models.ImageStatusError,
			}
			db.DB.Create(&imageSuccess)
			db.DB.Create(&imageError)
		})
		When("filter by name", func() {
			It("should return given image-set", func() {
				name := "image-set-1"
				req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets?name=%s", name), nil)
				Expect(err).ToNot(HaveOccurred())
				w := httptest.NewRecorder()
				req = req.WithContext(dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{}))
				handler := http.HandlerFunc(ListAllImageSets)
				handler.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK), fmt.Sprintf("expected status %d, but got %d", w.Code, http.StatusOK))
				respBody, err := ioutil.ReadAll(w.Body)
				Expect(err).To(BeNil())
				Expect(string(respBody)).To(ContainSubstring(name))
				Expect(string(respBody)).ToNot(ContainSubstring("image-set-2"))
			})
		})
		When("filter by status", func() {
			It("should return image-sets with ERROR status", func() {
				status := "ERROR"
				req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets?status=%s", status), nil)
				Expect(err).ToNot(HaveOccurred())
				w := httptest.NewRecorder()
				req = req.WithContext(dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{}))
				handler := http.HandlerFunc(ListAllImageSets)
				handler.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK), fmt.Sprintf("expected status %d, but got %d", w.Code, http.StatusOK))
				respBody, err := ioutil.ReadAll(w.Body)
				Expect(err).To(BeNil())
				Expect(string(respBody)).To(ContainSubstring("image-set-2"))
				Expect(string(respBody)).ToNot(ContainSubstring("image-set-1"))
			})
		})
	})

	Context("Installer ISO url", func() {
		var ctrl *gomock.Controller
		var router chi.Router
		var mockImageService *mock_services.MockImageServiceInterface
		var edgeAPIServices *dependencies.EdgeAPIServices

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			edgeAPIServices = &dependencies.EdgeAPIServices{
				ImageService: mockImageService,
				Log:          log.NewEntry(log.StandardLogger()),
			}
			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/image-sets", MakeImageSetsRouter)
		})

		It("getStorageInstallerIsoURL return the right iso path", func() {
			isoPath := getStorageInstallerIsoURL(120)
			Expect(isoPath).To(Equal("/api/edge/v1/storage/isos/120"))
		})

		When("getting imageSets", func() {
			orgID := common.DefaultOrgID
			imageName := faker.Name()
			installer1 := models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL()}
			db.DB.Create(&installer1)
			installer2 := models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL()}
			db.DB.Create(&installer2)

			imageSet := models.ImageSet{Name: imageName, OrgID: orgID}
			db.DB.Create(&imageSet)
			image1 := models.Image{Name: imageName, OrgID: orgID, ImageSetID: &imageSet.ID, Installer: &installer1}
			db.DB.Create(&image1)
			image2 := models.Image{Name: imageName, OrgID: orgID, ImageSetID: &imageSet.ID, Installer: &installer2}
			db.DB.Create(&image2)

			isoPathTemplate := "/api/edge/v1/storage/isos/%d"

			type AllImageSetsResponse struct {
				Count int                    `json:"Count"`
				Data  []ImageSetInstallerURL `json:"Data"`
			}
			type ImageSetDetailsResponse struct {
				Count int                   `json:"Count"`
				Data  ImageSetImagePackages `json:"Data"`
			}

			It("The app internal iso urls are used when getting all imageSets", func() {
				req, err := http.NewRequest("GET", "/image-sets/", nil)
				Expect(err).ToNot(HaveOccurred())
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))

				var allImageSetsResponse AllImageSetsResponse
				respBody, err := ioutil.ReadAll(rr.Body)
				err = json.Unmarshal(respBody, &allImageSetsResponse)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(allImageSetsResponse.Data) > 0).To(BeTrue())
				testImageFound := false
				for _, imageSetResponse := range allImageSetsResponse.Data {
					if imageSetResponse.ImageSetData.Name == imageName {
						testImageFound = true
						// the url should be the path of latest install
						Expect(*imageSetResponse.ImageBuildISOURL).To(Equal(fmt.Sprintf(isoPathTemplate, installer2.ID)))
						// it must have 2 images
						images := imageSetResponse.ImageSetData.Images
						Expect(len(images)).To(Equal(2))
						// the images are listed by latest first e.g. createdAtr Desc
						// and accordingly the latest installer must be first
						for ind, expectedImage := range []models.Image{image2, image1} {
							Expect(images[ind].ID).To(Equal(expectedImage.ID))
							Expect(images[ind].Installer.ImageBuildISOURL).To(Equal(fmt.Sprintf(isoPathTemplate, expectedImage.Installer.ID)))
						}
					}
				}
				// ensure the imageSet has been found and tested
				Expect(testImageFound).To(BeTrue())
			})
			It("The app internal iso urls are used when getting imageSet details", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/image-sets/%d", imageSet.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockImageService.EXPECT().AddPackageInfo(gomock.Any()).Return(services.ImageDetail{Image: &image1}, nil)
				mockImageService.EXPECT().AddPackageInfo(gomock.Any()).Return(services.ImageDetail{Image: &image2}, nil)

				rr := httptest.NewRecorder()

				router.ServeHTTP(rr, req)
				Expect(rr.Code).To(Equal(http.StatusOK))
				var imageSetDetailsResponse ImageSetDetailsResponse
				respBody, err := ioutil.ReadAll(rr.Body)
				err = json.Unmarshal(respBody, &imageSetDetailsResponse)
				Expect(err).ToNot(HaveOccurred())
				Expect(imageSetDetailsResponse.Count).To(Equal(2))
				Expect(imageSetDetailsResponse.Data.ImageBuildISOURL).To(Equal(fmt.Sprintf(isoPathTemplate, installer2.ID)))
				imageDetails := imageSetDetailsResponse.Data.Images
				Expect(len(imageDetails)).To(Equal(2))

				for ind, expectedImage := range []models.Image{image1, image2} {
					Expect(imageDetails[ind].Image.Installer.ImageBuildISOURL).To(Equal(fmt.Sprintf(isoPathTemplate, expectedImage.Installer.ID)))
				}
			})
		})
	})

})
