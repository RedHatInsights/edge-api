package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/platform-go-middlewares/identity"

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
	orgID := faker.UUIDHyphenated()
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
				Name:  "image-set-same-group",
				OrgID: orgID,
			}
			imageSetDifferentGroup := &models.ImageSet{
				Name:  "image-set-different-group",
				OrgID: orgID,
			}
			db.DB.Create(&imageSetSameGroup)
			db.DB.Create(&imageSetDifferentGroup)

			imageSameGroup1 = models.Image{
				Name:       "image-same-group-1",
				ImageSetID: &imageSetSameGroup.ID,
				Account:    "0000000",
				OrgID:      "0000000",
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
		When("when images selection has no image", func() {
			It("should return bad request", func() {
				jsonImagesBytes, err := json.Marshal([]models.Image{})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonImagesBytes))
				Expect(err).To(BeNil())

				rr := httptest.NewRecorder()
				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(PostValidateUpdate)

				handler.ServeHTTP(rr, req.WithContext(ctx))

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
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
					OrgID:         "111111",
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

	Context("Devices update", func() {
		var edgeAPIServices *dependencies.EdgeAPIServices
		var mockUpdateService *mock_services.MockUpdateServiceInterface
		var ctrl *gomock.Controller

		account := common.DefaultAccount
		orgID := common.DefaultOrgID
		account2 := faker.UUIDHyphenated()
		orgID2 := faker.UUIDHyphenated()

		imageSet := models.ImageSet{
			Account: account,
			OrgID:   orgID,
		}
		db.DB.Create(&imageSet)

		commit := models.Commit{
			Account: account,
			OrgID:   orgID,
		}
		db.DB.Create(&commit)

		image := models.Image{
			Account:    account,
			OrgID:      orgID,
			CommitID:   commit.ID,
			Status:     models.ImageStatusSuccess,
			Version:    1,
			ImageSetID: &imageSet.ID,
		}
		db.DB.Create(&image)

		device := models.Device{
			Account: account,
			OrgID:   orgID,
			UUID:    faker.UUIDHyphenated(),
			ImageID: image.ID,
		}
		db.DB.Create(&device)

		updateCommit := models.Commit{
			Account: account,
			OrgID:   orgID,
		}
		db.DB.Create(&updateCommit)

		updateImage := models.Image{
			Account:    account,
			OrgID:      orgID,
			CommitID:   updateCommit.ID,
			Status:     models.ImageStatusSuccess,
			Version:    2,
			ImageSetID: &imageSet.ID,
		}
		db.DB.Create(&updateImage)

		// a device from another account
		device2 := models.Device{Account: account2, OrgID: orgID2, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device2)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockUpdateService = mock_services.NewMockUpdateServiceInterface(ctrl)
			ctx := context.Background()
			logger := log.NewEntry(log.StandardLogger())

			edgeAPIServices = &dependencies.EdgeAPIServices{
				DeviceService: services.NewDeviceService(ctx, logger),
				CommitService: services.NewCommitService(ctx, logger),
				UpdateService: mockUpdateService,
				Log:           logger,
			}
		})
		AfterEach(func() {
			ctrl.Finish()
		})

		When("when devices does not exists", func() {

			It("should not allow to update", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID, "does-not-exists"}})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)

				responseRecorder := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
			})

			It("should not allow to update when devices from different accounts", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID, device2.UUID}})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)

				responseRecorder := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("when devices exists and update commit exists", func() {
			updateTransactions := []models.UpdateTransaction{
				{Account: account, OrgID: orgID, CommitID: updateCommit.ID, Devices: []models.Device{device}, Status: models.UpdateStatusBuilding},
			}
			db.DB.Create(updateTransactions)

			It("should allow to update without commitID", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID}})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := req.Context()
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), account, orgID, gomock.Any()).Return(&updateTransactions, nil)
				mockUpdateService.EXPECT().CreateUpdateAsync(updateTransactions[0].ID)

				responseRecorder := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("should allow to update with commitID", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{CommitID: updateCommit.ID, DevicesUUID: []string{device.UUID}})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := req.Context()
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), account, orgID, gomock.Any()).Return(&updateTransactions, nil)
				mockUpdateService.EXPECT().CreateUpdateAsync(updateTransactions[0].ID)

				responseRecorder := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})
		})
	})
})
