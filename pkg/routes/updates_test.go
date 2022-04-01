package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

func TestGetUpdateByID(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), UpdateContextKey, &testUpdates[0])
	handler := http.HandlerFunc(GetUpdateByID)
	handler.ServeHTTP(rr, req.WithContext(ctx))

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}

	var ir models.UpdateTransaction
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}

	err = json.Unmarshal(respBody, &ir)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ir.ID != testUpdates[0].ID {
		t.Errorf("wrong image status: got %v want %v",
			ir.ID, testImage.ID)
	}
}

func TestGetUpdatePlaybook(t *testing.T) {
	// Given
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	update := &testUpdates[0]
	ctx := context.WithValue(req.Context(), UpdateContextKey, update)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	playbookString := "mocked playbook"
	reader := ioutil.NopCloser(strings.NewReader(playbookString))
	mockUpdateService := mock_services.NewMockUpdateServiceInterface(ctrl)
	mockUpdateService.EXPECT().GetUpdatePlaybook(gomock.Eq(update)).Return(reader, nil)
	ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		UpdateService: mockUpdateService,
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetUpdatePlaybook)

	// When
	handler.ServeHTTP(rr, req.WithContext(ctx))

	// Then
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return
	}
	respBody, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Errorf(err.Error())
	}
	if string(respBody) != playbookString {
		t.Errorf("wrong response: got %v want %v",
			respBody, playbookString)
		return
	}

}

var _ = Describe("Update routes", func() {
	var edgeAPIServices *dependencies.EdgeAPIServices
	BeforeEach(func() {
		logger := log.NewEntry(log.StandardLogger())
		edgeAPIServices = &dependencies.EdgeAPIServices{
			UpdateService: services.NewUpdateService(context.Background(), logger),
			Log:           logger,
		}
	})
	Context("POST PostValidateUpdate", func() {
		var imageSameGroup1 models.Image
		var imageSameGroup2 models.Image
		var imageDifferentGroup models.Image

		BeforeEach(func() {
			imageSetSameGroup := &models.ImageSet{
				Name: "image-set-same-group",
			}
			imageSetDifferentGroup := &models.ImageSet{
				Name: "image-set-different-group",
			}
			db.DB.Create(&imageSetSameGroup)
			db.DB.Create(&imageSetDifferentGroup)

			imageSameGroup1 = models.Image{
				Name:       "image-same-group-1",
				ImageSetID: &imageSetSameGroup.ID,
				Account:    "0000000",
			}
			imageSameGroup2 = imageSameGroup1
			imageSameGroup2.Name = "image-same-group-2"
			imageDifferentGroup = imageSameGroup1
			imageDifferentGroup.Name = "image-different-group"
			imageDifferentGroup.ImageSetID = &imageSetDifferentGroup.ID
			db.DB.Create(&imageSameGroup1)
			db.DB.Create(&imageSameGroup2)
			db.DB.Create(&imageDifferentGroup)
		})
		When("when images selection has one image", func() {
			It("should allow to update", func() {
				jsonImagesBytes, err := json.Marshal([]models.Image{imageSameGroup1})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonImagesBytes))
				Expect(err).To(BeNil())

				rr := httptest.NewRecorder()
				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(PostValidateUpdate)

				handler.ServeHTTP(rr, req.WithContext(ctx))

				jsonResponse, _ := json.Marshal(ValidateUpdateResponse{UpdateValid: true})

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).Should(MatchJSON(jsonResponse))
			})
		})
		When("when images selection has the same image set and same account", func() {
			It("should allow to update", func() {
				jsonImagesBytes, err := json.Marshal([]models.Image{imageSameGroup1, imageSameGroup2})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonImagesBytes))
				Expect(err).To(BeNil())

				rr := httptest.NewRecorder()
				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(PostValidateUpdate)

				handler.ServeHTTP(rr, req.WithContext(ctx))

				jsonResponse, _ := json.Marshal(ValidateUpdateResponse{UpdateValid: true})

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).Should(MatchJSON(jsonResponse))
			})
		})
		When("when images selection has the same image set and different account", func() {
			BeforeEach(func() {
				config.Get().Auth = true
			})
			AfterEach(func() {
				config.Get().Auth = false
			})
			It("should not allow to update", func() {
				jsonImagesBytes, err := json.Marshal([]models.Image{imageSameGroup1, imageSameGroup2})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonImagesBytes))
				Expect(err).To(BeNil())

				rr := httptest.NewRecorder()
				ctx := context.WithValue(req.Context(), identity.Key, identity.XRHID{Identity: identity.Identity{
					AccountNumber: "111111",
				}})
				req = req.WithContext(ctx)
				ctx = dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(PostValidateUpdate)

				handler.ServeHTTP(rr, req.WithContext(ctx))

				jsonResponse, _ := json.Marshal(ValidateUpdateResponse{UpdateValid: false})

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).Should(MatchJSON(jsonResponse))
			})
		})
		When("when images selection has different image sets", func() {
			It("should not allow to update", func() {
				jsonImagesBytes, err := json.Marshal([]models.Image{imageSameGroup1, imageDifferentGroup})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonImagesBytes))
				Expect(err).To(BeNil())

				rr := httptest.NewRecorder()
				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(PostValidateUpdate)

				handler.ServeHTTP(rr, req.WithContext(ctx))

				jsonResponse, _ := json.Marshal(ValidateUpdateResponse{UpdateValid: false})

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(rr.Body.String()).Should(MatchJSON(jsonResponse))
			})
		})
	})
})
