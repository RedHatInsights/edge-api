// FIXME: golangci-lint
// nolint:errcheck,govet,ineffassign,revive,staticcheck,typecheck
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	testHelpers "github.com/redhatinsights/edge-api/internal/testing"
	apiError "github.com/redhatinsights/edge-api/pkg/errors"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/clients/inventorygroups/mock_inventorygroups"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/go-chi/chi"
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
	ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{})
	ctx = context.WithValue(ctx, UpdateContextKey, &testUpdates[0])
	rr := httptest.NewRecorder()
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
				OrgID:      common.DefaultOrgID,
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
		When("when images selection has the same image set and same orgID", func() {
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
		When("when images selection has the same image set and different orgID", func() {
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
				ctx := testHelpers.WithCustomIdentity(req.Context(), "111111")
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
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

		orgID := common.DefaultOrgID
		orgID2 := faker.UUIDHyphenated()

		imageSet := models.ImageSet{
			OrgID: orgID,
		}
		db.DB.Create(&imageSet)

		commit := models.Commit{
			OrgID: orgID,
		}
		db.DB.Create(&commit)

		image := models.Image{
			OrgID:      orgID,
			CommitID:   commit.ID,
			Status:     models.ImageStatusSuccess,
			Version:    1,
			ImageSetID: &imageSet.ID,
		}
		db.DB.Create(&image)

		device := models.Device{
			OrgID:   orgID,
			UUID:    faker.UUIDHyphenated(),
			ImageID: image.ID,
		}
		db.DB.Create(&device)

		updateCommit := models.Commit{
			OrgID: orgID,
		}
		db.DB.Create(&updateCommit)

		updateImage := models.Image{
			OrgID:      orgID,
			CommitID:   updateCommit.ID,
			Status:     models.ImageStatusSuccess,
			Version:    2,
			ImageSetID: &imageSet.ID,
		}
		db.DB.Create(&updateImage)

		imageSet2 := models.ImageSet{
			OrgID: orgID,
		}
		db.DB.Create(&imageSet2)

		// a Image with ImageSet from imageSet2
		imageWithImageSetID := models.Image{
			OrgID:      orgID,
			ImageSetID: &imageSet2.ID,
			Status:     models.ImageStatusSuccess,
			CommitID:   commit.ID,
		}
		db.DB.Create(&imageWithImageSetID)

		// a device3
		device3 := models.Device{
			OrgID:   orgID,
			UUID:    faker.UUIDHyphenated(),
			ImageID: imageWithImageSetID.ID,
		}
		db.DB.Create(&device3)

		// a device from another orgID
		device2 := models.Device{OrgID: orgID2, UUID: faker.UUIDHyphenated()}
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

			It("should respond http status ok when build transaction is empty", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID}})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := req.Context()
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), orgID, gomock.Any()).Return(&[]models.UpdateTransaction{}, nil)

				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(rr, req)

				var response common.APIResponse
				respBody, _ := ioutil.ReadAll(rr.Body)
				err = json.Unmarshal(respBody, &response)

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(err).Should(BeNil())
				Expect(response.Message).To(Equal("There are no updates to perform"))
			})

			It("should not allow to update when devices from different orgID", func() {
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
				{OrgID: orgID, CommitID: updateCommit.ID, Devices: []models.Device{device}, Status: models.UpdateStatusBuilding},
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

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), orgID, gomock.Any()).Return(&updateTransactions, nil)
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

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), orgID, gomock.Any()).Return(&updateTransactions, nil)
				mockUpdateService.EXPECT().CreateUpdateAsync(updateTransactions[0].ID)

				responseRecorder := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})
		})
		When("CommitID provided by user does not belong to same ImageSet as that of Device Image", func() {
			It("should not allow to update with commitID belonging to different ImageSet", func() {

				updateData, err := json.Marshal(models.DevicesUpdate{CommitID: updateCommit.ID, DevicesUUID: []string{device3.UUID}})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)

				responseRecorder := httptest.NewRecorder()

				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				respBody, err := ioutil.ReadAll(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Commit %d does not belong to the same image-set as devices", updateCommit.ID))
			})
		})
		When("CommitID provided by user does not exist", func() {
			It("should not allow to update with commitID that dose not exist", func() {
				non_existant_commit := uint(99999999)
				updateData, err := json.Marshal(models.DevicesUpdate{CommitID: non_existant_commit, DevicesUUID: []string{device3.UUID}})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)

				responseRecorder := httptest.NewRecorder()

				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				respBody, err := ioutil.ReadAll(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("No commit found for CommitID %d", non_existant_commit))
			})
		})

		When("CommitID is not valid for update because is rollback info", func() {

			It("should not allow to update with prior commitID ", func() {
				// a deviceRunning latest image
				deviceUpdated := models.Device{
					OrgID:   orgID,
					UUID:    faker.UUIDHyphenated(),
					ImageID: updateImage.ID,
				}
				errDB := db.DB.Create(&deviceUpdated)

				Expect(errDB.Error).To(BeNil())

				updateData, err := json.Marshal(models.DevicesUpdate{CommitID: image.CommitID, DevicesUUID: []string{deviceUpdated.UUID}})
				Expect(err).To(BeNil())

				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)

				responseRecorder := httptest.NewRecorder()

				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(responseRecorder, req)

				respBody, err := ioutil.ReadAll(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())

				var response apiError.BadRequest
				err = json.Unmarshal(respBody, &response)
				Expect(response.Title).To(Equal(fmt.Sprintf("Commit %d is not valid for update", image.CommitID)))
			})
		})
	})

	Context("Devices update by diff commits", func() {
		var edgeAPIServices *dependencies.EdgeAPIServices
		var mockUpdateService *mock_services.MockUpdateServiceInterface
		var ctrl *gomock.Controller

		orgID := common.DefaultOrgID

		imageSet := models.ImageSet{
			OrgID: orgID,
		}
		db.DB.Create(&imageSet)

		commits := []models.Commit{{OrgID: orgID, Name: "1"},
			{OrgID: orgID, Name: "2"},
			{OrgID: orgID, Name: "3"},
			{OrgID: orgID, Name: "4"},
			{OrgID: orgID, Name: "5"}}
		db.DB.Create(&commits)

		images := [5]models.Image{
			{OrgID: orgID, CommitID: commits[0].ID, Status: models.ImageStatusSuccess, Version: 1, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[1].ID, Status: models.ImageStatusSuccess, Version: 2, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[2].ID, Status: models.ImageStatusSuccess, Version: 3, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[3].ID, Status: models.ImageStatusSuccess, Version: 4, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[4].ID, Status: models.ImageStatusSuccess, Version: 5, ImageSetID: &imageSet.ID},
		}
		db.DB.Create(&images)

		device := models.Device{
			OrgID:   orgID,
			UUID:    faker.UUIDHyphenated(),
			ImageID: images[1].ID,
		}
		db.DB.Create(&device)
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

		When("when try to update a device to specific image", func() {

			It("should not allow to update to previous commit", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID}, CommitID: commits[0].ID})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := dependencies.ContextWithServices(req.Context(), edgeAPIServices)
				req = req.WithContext(ctx)

				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(rr, req)

				var response common.APIResponse
				respBody, _ := ioutil.ReadAll(rr.Body)
				_ = json.Unmarshal(respBody, &response)

				responseRecorder := httptest.NewRecorder()
				handler.ServeHTTP(responseRecorder, req)
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))

			})

			It("should update to version 3 and see 4,5 available", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID}, CommitID: commits[2].ID})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := req.Context()
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				var desiredCommit models.Commit
				db.DB.First(&desiredCommit, &commits[2].ID)
				mockUpdateService.EXPECT().BuildUpdateTransactions(&models.DevicesUpdate{DevicesUUID: []string{device.UUID}, CommitID: commits[2].ID},
					orgID, &desiredCommit).
					Return(&[]models.UpdateTransaction{}, nil)
				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(rr, req)

				var response common.APIResponse
				respBody, _ := ioutil.ReadAll(rr.Body)
				err = json.Unmarshal(respBody, &response)

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(err).Should(BeNil())
			})

			It("should update to version 4 and see only 5 available", func() {
				updateData, err := json.Marshal(models.DevicesUpdate{DevicesUUID: []string{device.UUID}, CommitID: commits[3].ID})
				Expect(err).To(BeNil())
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(updateData))
				Expect(err).To(BeNil())

				ctx := req.Context()
				ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
				req = req.WithContext(ctx)
				var desiredCommit models.Commit
				db.DB.First(&desiredCommit, &commits[3].ID)
				mockUpdateService.EXPECT().BuildUpdateTransactions(&models.DevicesUpdate{DevicesUUID: []string{device.UUID}, CommitID: commits[3].ID},
					orgID, &desiredCommit).
					Return(&[]models.UpdateTransaction{}, nil)

				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(rr, req)

				var response common.APIResponse
				respBody, _ := ioutil.ReadAll(rr.Body)
				err = json.Unmarshal(respBody, &response)

				Expect(rr.Code).To(Equal(http.StatusOK))
				Expect(err).Should(BeNil())
			})

		})

	})

	Context("get all updates with filter parameters", func() {
		tt := []struct {
			name          string
			params        string
			expectedError []validationError
		}{
			{
				name:   "bad created_at date",
				params: "created_at=today",
				expectedError: []validationError{
					{Key: "created_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
				},
			},
			{
				name:   "bad sort_by",
				params: "sort_by=test",
				expectedError: []validationError{
					{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must be name or created_at or updated_at"},
				},
			},
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		for _, te := range tt {
			req, err := http.NewRequest("GET", fmt.Sprintf("/updates?%s", te.params), nil)
			Expect(err).ToNot(HaveOccurred())
			w := httptest.NewRecorder()

			ValidateGetAllDeviceGroupsFilterParams(next).ServeHTTP(w, req)

			resp := w.Result()
			var jsonBody []validationError
			err = json.NewDecoder(resp.Body).Decode(&jsonBody)
			Expect(err).ToNot(HaveOccurred())
			for _, exErr := range te.expectedError {
				found := false
				for _, jsErr := range jsonBody {
					if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody))
			}
		}
	})

	Context("ValidateGetUpdatesFilterParams", func() {
		var ctrl *gomock.Controller
		var mockImageService *mock_services.MockImageServiceInterface

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("filters and sort_by working as expected", func() {

			tt := []struct {
				name          string
				params        string
				expectedError []validationError
			}{
				{
					name:          "good ASC sort_by",
					params:        "sort_by=updated_at",
					expectedError: nil,
				},
				{
					name:          "good DESC sort_by",
					params:        "sort_by=-updated_at",
					expectedError: nil,
				},
				{
					name:   "bad created_at date",
					params: "created_at=today",
					expectedError: []validationError{
						{Key: "created_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
					},
				},
				{
					name:   "bad created_at date",
					params: "updated_at=today",
					expectedError: []validationError{
						{Key: "updated_at", Reason: `parsing time "today" as "2006-01-02": cannot parse "today" as "2006"`},
					},
				},
				{
					name:   "bad sort_by",
					params: "sort_by=test",
					expectedError: []validationError{
						{Key: "sort_by", Reason: "test is not a valid sort_by. Sort-by must be created_at or updated_at"},
					},
				},
			}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			for _, te := range tt {
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates?%s", te.params), nil)
				Expect(err).ToNot(HaveOccurred())
				w := httptest.NewRecorder()

				ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
					ImageService: mockImageService,
					Log:          log.NewEntry(log.StandardLogger()),
				})
				req = req.WithContext(ctx)

				ValidateGetUpdatesFilterParams(next).ServeHTTP(w, req)
				if te.expectedError == nil {
					Expect(w.Code).To(Equal(http.StatusOK))
					continue
				}
				Expect(w.Code).To(Equal(http.StatusBadRequest))
				resp := w.Result()
				var jsonBody []validationError
				err = json.NewDecoder(resp.Body).Decode(&jsonBody)
				Expect(err).ToNot(HaveOccurred())
				for _, exErr := range te.expectedError {
					found := false
					for _, jsErr := range jsonBody {
						if jsErr.Key == exErr.Key && jsErr.Reason == exErr.Reason {
							found = true
							break
						}
					}
					Expect(found).To(BeTrue(), fmt.Sprintf("in %q: was expected to have %v but not found in %v", te.name, exErr, jsonBody))
				}
			}
		})
	})

	Context("update end-points", func() {

		var ctrl *gomock.Controller
		var router chi.Router
		var mockUpdateService *mock_services.MockUpdateServiceInterface
		var edgeAPIServices *dependencies.EdgeAPIServices

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockUpdateService = mock_services.NewMockUpdateServiceInterface(ctrl)
			edgeAPIServices = &dependencies.EdgeAPIServices{
				UpdateService: mockUpdateService,
				Log:           log.NewEntry(log.StandardLogger()),
			}
			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/updates", MakeUpdatesRouter)
		})
		AfterEach(func() {
			ctrl.Finish()
		})

		Context("GetUpdatePlaybook", func() {

			updateTransaction := models.UpdateTransaction{
				OrgID: common.DefaultOrgID,
				Repo:  &models.Repo{URL: faker.URL(), Status: models.ImageStatusSuccess},
			}
			res := db.DB.Create(&updateTransaction)

			It("should return the requested resource content", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d/update-playbook.yml", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				fileContent := "this is a simple file content"

				fileContentReader := strings.NewReader(fileContent)
				fileContentReadCloser := io.NopCloser(fileContentReader)
				mockUpdateService.EXPECT().GetUpdatePlaybook(gomock.Any()).Return(fileContentReadCloser, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(Equal(fileContent))
			})

			It("should return error UpdateService.GetUpdatePlaybook return error", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d/update-playbook.yml", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedError := errors.New("error resource not found")
				mockUpdateService.EXPECT().GetUpdatePlaybook(gomock.Any()).Return(nil, expectedError)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("file was not found on the S3 bucket"))
			})

			It("should return error when updateTransaction does not exist in context", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d/update-playbook.yml", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()

				ctx := dependencies.ContextWithServices(context.Background(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(GetUpdatePlaybook)

				handler.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update-transaction not found in context"))
			})
		})

		Context("GetUpdates", func() {
			It("should return update transactions", func() {
				updateTransaction1 := models.UpdateTransaction{
					OrgID: common.DefaultOrgID,
					Repo:  &models.Repo{URL: faker.URL(), Status: models.ImageStatusSuccess},
				}
				res1 := db.DB.Create(&updateTransaction1)
				updateTransaction2 := models.UpdateTransaction{
					OrgID: common.DefaultOrgID,
					Repo:  &models.Repo{URL: faker.URL(), Status: models.ImageStatusSuccess},
				}
				res2 := db.DB.Create(&updateTransaction2)

				Expect(res1.Error).ToNot(HaveOccurred())
				Expect(res2.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", "/updates?sort_by=-created_at&limit=2", nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var respUpdates []models.UpdateTransaction
				err = json.Unmarshal(respBody, &respUpdates)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(respUpdates)).To(Equal(2))
				Expect(respUpdates[0].ID).To(Equal(updateTransaction2.ID))
				Expect(respUpdates[1].ID).To(Equal(updateTransaction1.ID))
			})
			Context("empty org", func() {
				originalAuth := config.Get().Auth
				BeforeEach(func() {
					config.Get().Auth = true
				})

				AfterEach(func() {
					config.Get().Auth = originalAuth
				})

				It("should return error when org is not set  in context", func() {
					req, err := http.NewRequest("GET", "/updates", nil)
					Expect(err).ToNot(HaveOccurred())

					httpTestRecorder := httptest.NewRecorder()

					ctx := dependencies.ContextWithServices(context.Background(), edgeAPIServices)
					ctx = testHelpers.WithCustomIdentity(ctx, "")
					handler := http.HandlerFunc(GetUpdates)

					handler.ServeHTTP(httpTestRecorder, req.WithContext(ctx))

					Expect(httpTestRecorder.Code).To(Equal(http.StatusBadRequest))
					respBody, err := io.ReadAll(httpTestRecorder.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(respBody)).To(ContainSubstring("cannot find org-id"))
				})
			})
		})

		Context("GetUpdateByID", func() {
			updateTransaction := models.UpdateTransaction{
				OrgID: common.DefaultOrgID,
				Repo:  &models.Repo{URL: faker.URL(), Status: models.ImageStatusSuccess},
			}
			res := db.DB.Create(&updateTransaction)

			It("should return update transaction successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var respUpdate models.UpdateTransaction
				err = json.Unmarshal(respBody, &respUpdate)
				Expect(err).ToNot(HaveOccurred())
				Expect(respUpdate.ID).To(Equal(updateTransaction.ID))
			})

			It("should return error when transaction does not exist", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d", 9999999999), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update not found"))
			})

			It("should return error when updateTransaction does not exist in context", func() {
				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				httpTestRecorder := httptest.NewRecorder()

				ctx := dependencies.ContextWithServices(context.Background(), edgeAPIServices)
				req = req.WithContext(ctx)
				handler := http.HandlerFunc(GetUpdateByID)

				handler.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusNotFound))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("update-transaction not found in context"))
			})

		})

		Context("SendNotificationForDevice", func() {
			updateTransaction := models.UpdateTransaction{
				OrgID: common.DefaultOrgID,
				Repo:  &models.Repo{URL: faker.URL(), Status: models.ImageStatusSuccess},
			}
			res := db.DB.Create(&updateTransaction)

			notification := services.ImageNotification{
				Version:     services.NotificationConfigVersion,
				Bundle:      services.NotificationConfigBundle,
				Application: services.NotificationConfigApplication,
				EventType:   services.NotificationConfigEventTypeImage,
				Timestamp:   time.Now().Format(time.RFC3339),
			}

			It("should send notification successfully", func() {
				Expect(res.Error).ToNot(HaveOccurred())

				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d/notify", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				mockUpdateService.EXPECT().SendDeviceNotification(gomock.Any()).Return(notification, nil)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusOK))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).ToNot(BeEmpty())

				var resNotification services.ImageNotification
				err = json.Unmarshal(respBody, &resNotification)
				Expect(err).ToNot(HaveOccurred())
				Expect(resNotification.Version).To(Equal(notification.Version))
				Expect(resNotification.EventType).To(Equal(notification.EventType))
			})
			It("should return error when send notification failed", func() {
				Expect(res.Error).ToNot(HaveOccurred())

				req, err := http.NewRequest("GET", fmt.Sprintf("/updates/%d/notify", updateTransaction.ID), nil)
				Expect(err).ToNot(HaveOccurred())

				expectedErr := errors.New("notification error")
				mockUpdateService.EXPECT().SendDeviceNotification(gomock.Any()).Return(notification, expectedErr)

				httpTestRecorder := httptest.NewRecorder()
				router.ServeHTTP(httpTestRecorder, req)

				Expect(httpTestRecorder.Code).To(Equal(http.StatusInternalServerError))
				respBody, err := io.ReadAll(httpTestRecorder.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(respBody)).To(ContainSubstring("Failed to send notification"))
			})
		})
	})
})

func TestValidateGetAllUpdatesQueryParameters(t *testing.T) {
	tt := []struct {
		name          string
		params        string
		expectedError []validationError
	}{
		{
			name:   "invalid query param",
			params: "bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("updates"))},
			},
		},
		{
			name:   "valid query param and invalid query param",
			params: "sort_by=created_at&bla=1",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("updates"))},
			},
		},
		{
			name:   "invalid query param and valid query param",
			params: "bla=1&sort_by=created_at",
			expectedError: []validationError{
				{Key: "bla", Reason: fmt.Sprintf("bla is not a valid query param, supported query params: %s", GetQueryParamsArray("updates"))},
			},
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	for _, te := range tt {
		req, err := http.NewRequest("GET", fmt.Sprintf("/updates?%s", te.params), nil)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockImageService := mock_services.NewMockImageServiceInterface(ctrl)
		ctx := dependencies.ContextWithServices(req.Context(), &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
			Log:          log.NewEntry(log.StandardLogger()),
		})
		req = req.WithContext(ctx)

		ValidateQueryParams("updates")(next).ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
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

func TestInventoryGroupDevicesUpdateInfo(t *testing.T) {

	defer func() {
		config.Get().Auth = false
	}()

	// enable auth
	config.Get().Auth = true

	orgID := faker.UUIDHyphenated()
	groupUUID := faker.UUIDHyphenated()
	inventoryGroup := inventorygroups.Group{Name: faker.Name(), ID: groupUUID, OrgID: orgID}
	expectedError := errors.New("some expected error")
	testCases := []struct {
		Name                             string
		EnforceEdgeGroups                bool
		EdgeParityInventoryGroupsEnabled bool
		GroupUUID                        string
		ReturnInventoryGroup             *inventorygroups.Group
		ReturnInventoryGroupError        error
		ReturnServiceError               error
		ReturnServiceData                *models.InventoryGroupDevicesUpdateInfo
		ExpectedHTTPStatus               int
		ExpectedHTTPErrorMessage         string
	}{
		{
			Name:                             "should return InventoryGroupDevicesUpdateInfo successfully",
			EdgeParityInventoryGroupsEnabled: true,
			GroupUUID:                        groupUUID,
			ReturnInventoryGroup:             &inventoryGroup,
			ReturnServiceData:                &models.InventoryGroupDevicesUpdateInfo{UpdateValid: true, DevicesUUIDS: []string{faker.UUIDHyphenated()}},
			ExpectedHTTPStatus:               http.StatusOK,
		},
		{
			Name:                     "should return bad request error when inventory group not supplied",
			GroupUUID:                "",
			ExpectedHTTPStatus:       http.StatusBadRequest,
			ExpectedHTTPErrorMessage: "missing inventory group uuid",
		},
		{
			Name:                      "should return not found error when inventory group not found",
			GroupUUID:                 groupUUID,
			ReturnInventoryGroup:      nil,
			ReturnInventoryGroupError: inventorygroups.ErrGroupNotFound,
			ExpectedHTTPStatus:        http.StatusNotFound,
			ExpectedHTTPErrorMessage:  "inventory group not found",
		},
		{
			Name:                      "should return internal server error when inventory group return unknown error",
			GroupUUID:                 groupUUID,
			ReturnInventoryGroup:      nil,
			ReturnInventoryGroupError: expectedError,
			ExpectedHTTPStatus:        http.StatusInternalServerError,
		},
		{
			Name:                             "should return error when inventory groups feature is not in use",
			EdgeParityInventoryGroupsEnabled: false,
			GroupUUID:                        groupUUID,
			ReturnInventoryGroup:             &inventoryGroup,
			ReturnInventoryGroupError:        nil,
			ExpectedHTTPStatus:               http.StatusNotImplemented,
			ExpectedHTTPErrorMessage:         "inventory groups feature is not available",
		},
		{
			Name:                             "should return error when EdgeGroups is enforced",
			EdgeParityInventoryGroupsEnabled: true,
			EnforceEdgeGroups:                true,
			GroupUUID:                        groupUUID,
			ReturnInventoryGroup:             &inventoryGroup,
			ReturnInventoryGroupError:        nil,
			ExpectedHTTPStatus:               http.StatusNotImplemented,
			ExpectedHTTPErrorMessage:         "inventory groups feature is not available",
		},
		{
			Name:                             "should return error when InventoryGroupDevicesUpdateInfo fails",
			EdgeParityInventoryGroupsEnabled: true,
			GroupUUID:                        groupUUID,
			ReturnInventoryGroup:             &inventoryGroup,
			ReturnInventoryGroupError:        nil,
			ReturnServiceError:               expectedError,
			ExpectedHTTPStatus:               http.StatusInternalServerError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			RegisterTestingT(t)

			defer func() {
				_ = os.Unsetenv(feature.EnforceEdgeGroups.EnvVar)
				_ = os.Unsetenv(feature.EdgeParityInventoryGroupsEnabled.EnvVar)
			}()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			if testCase.EnforceEdgeGroups {
				err := os.Setenv(feature.EnforceEdgeGroups.EnvVar, "true")
				Expect(err).ToNot(HaveOccurred())
			}
			if testCase.EdgeParityInventoryGroupsEnabled {
				err := os.Setenv(feature.EdgeParityInventoryGroupsEnabled.EnvVar, "true")
				Expect(err).ToNot(HaveOccurred())
			}

			var router chi.Router
			var edgeAPIServices *dependencies.EdgeAPIServices

			mockUpdateService := mock_services.NewMockUpdateServiceInterface(ctrl)
			mockInventoryGroupsClient := mock_inventorygroups.NewMockClientInterface(ctrl)

			router = chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					rLog := log.NewEntry(log.StandardLogger())
					ctx := testHelpers.WithCustomIdentity(r.Context(), orgID)
					edgeAPIServices = &dependencies.EdgeAPIServices{
						UpdateService:          mockUpdateService,
						InventoryGroupsService: mockInventoryGroupsClient,
						Log:                    rLog,
					}
					ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)

					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			router.Route("/updates", MakeUpdatesRouter)

			req, err := http.NewRequest(
				http.MethodGet, fmt.Sprintf("/updates/inventory-groups/%s/update-info", testCase.GroupUUID), nil,
			)
			Expect(err).ToNot(HaveOccurred())

			if testCase.ReturnInventoryGroup != nil || testCase.ReturnInventoryGroupError != nil {
				mockInventoryGroupsClient.EXPECT().GetGroupByUUID(testCase.GroupUUID).Return(
					testCase.ReturnInventoryGroup, testCase.ReturnInventoryGroupError,
				)
			}

			if testCase.ReturnServiceData != nil || testCase.ReturnServiceError != nil {
				mockUpdateService.EXPECT().InventoryGroupDevicesUpdateInfo(orgID, testCase.GroupUUID).Return(
					testCase.ReturnServiceData, testCase.ReturnServiceError,
				)
			}
			responseRecorder := httptest.NewRecorder()
			router.ServeHTTP(responseRecorder, req)
			respBody, err := io.ReadAll(responseRecorder.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(respBody)).ToNot(BeEmpty())

			Expect(responseRecorder.Code).To(Equal(testCase.ExpectedHTTPStatus))
			if testCase.ExpectedHTTPStatus == http.StatusOK && testCase.ReturnServiceData != nil {
				var responseUpdateInfo models.InventoryGroupDevicesUpdateInfo
				err = json.Unmarshal(respBody, &responseUpdateInfo)
				Expect(err).ToNot(HaveOccurred())
				Expect(responseUpdateInfo.UpdateValid).To(Equal(testCase.ReturnServiceData.UpdateValid))
				Expect(responseUpdateInfo.DevicesUUIDS).To(Equal(testCase.ReturnServiceData.DevicesUUIDS))
			} else if testCase.ExpectedHTTPErrorMessage != "" {
				Expect(string(respBody)).To(ContainSubstring(testCase.ExpectedHTTPErrorMessage))
			}
		})
	}
}
