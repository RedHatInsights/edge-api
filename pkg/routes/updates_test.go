// FIXME: golangci-lint
// nolint:errcheck,govet,ineffassign,revive,staticcheck
package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
				ctx := context.WithValue(req.Context(), identity.Key, identity.XRHID{Identity: identity.Identity{
					OrgID: "111111",
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
				respBody, err := ioutil.ReadAll(rr.Body)
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
		db.DB.Debug().Create(&commits)

		images := [5]models.Image{
			{OrgID: orgID, CommitID: commits[0].ID, Status: models.ImageStatusSuccess, Version: 1, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[1].ID, Status: models.ImageStatusSuccess, Version: 2, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[2].ID, Status: models.ImageStatusSuccess, Version: 3, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[3].ID, Status: models.ImageStatusSuccess, Version: 4, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[4].ID, Status: models.ImageStatusSuccess, Version: 5, ImageSetID: &imageSet.ID},
		}
		db.DB.Debug().Create(&images)

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
				respBody, err := ioutil.ReadAll(rr.Body)
				err = json.Unmarshal(respBody, &response)

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

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), orgID, gomock.Any()).Return(&[]models.UpdateTransaction{}, nil)

				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(rr, req)

				var response common.APIResponse
				respBody, err := ioutil.ReadAll(rr.Body)
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

				mockUpdateService.EXPECT().BuildUpdateTransactions(gomock.Any(), orgID, gomock.Any()).Return(&[]models.UpdateTransaction{}, nil)

				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(AddUpdate)
				handler.ServeHTTP(rr, req)

				var response common.APIResponse
				respBody, err := ioutil.ReadAll(rr.Body)
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
