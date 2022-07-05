package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	log "github.com/sirupsen/logrus"
)

// MakeUpdatesRouter adds support for operations on update
func MakeUpdatesRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", GetUpdates)
	sub.Post("/", AddUpdate)
	sub.Post("/validate", PostValidateUpdate)
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateCtx)
		r.Get("/", GetUpdateByID)
		r.Get("/update-playbook.yml", GetUpdatePlaybook)
		r.Get("/notify", SendNotificationForDevice) //TMP ROUTE TO SEND THE NOTIFICATION
	})
	// TODO: This is for backwards compatibility with the previous route
	// Once the frontend starts querying the device
	sub.Route("/device/", MakeDevicesRouter)
}

type updateContextKey int

// UpdateContextKey is the key to Update Context handler
const UpdateContextKey updateContextKey = iota

// UpdateCtx is a handler for Update requests
func UpdateCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		var updates []models.UpdateTransaction
		account, orgID := readAccountOrOrgID(w, r, ctxServices.Log)
		if account == "" && orgID == "" {
			return
		}
		updateID := chi.URLParam(r, "updateID")
		ctxServices.Log = ctxServices.Log.WithField("updateID", updateID)
		if updateID == "" {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("UpdateTransactionID can't be empty"))
			return
		}
		id, err := strconv.Atoi(updateID)
		if err != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
			return
		}
		if result := db.AccountOrOrg(account, orgID, "update_transactions").Preload("DispatchRecords").Preload("Devices").Preload("OldCommits").
			Joins("Commit").Joins("Repo").Find(&updates, id); result.Error != nil {
			ctxServices.Log.WithFields(log.Fields{
				"error": result.Error.Error(),
			}).Error("Error retrieving updates")
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}
		if len(updates) == 0 {
			respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("update not found"))
			return
		}
		ctx := context.WithValue(r.Context(), UpdateContextKey, &updates[0])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUpdatePlaybook returns the playbook for a update transaction
func GetUpdatePlaybook(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		// Error set by UpdateCtx already
		return
	}
	services := dependencies.ServicesFromContext(r.Context())
	playbook, err := services.UpdateService.GetUpdatePlaybook(update)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error getting update playbook")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	defer playbook.Close()
	_, err = io.Copy(w, playbook)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error reading the update playbook")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
}

// GetUpdates returns the updates for the device
func GetUpdates(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	var updates []models.UpdateTransaction
	account, orgID := readAccountOrOrgID(w, r, services.Log)
	if account == "" && orgID == "" {
		return
	}
	if result := db.AccountOrOrg(account, orgID, "update_transactions").Preload("DispatchRecords").Preload("Devices").
		Joins("Commit").Joins("Repo").Find(&updates); result.Error != nil {
		services.Log.WithFields(log.Fields{
			"error": result.Error.Error(),
		}).Error("Error retrieving updates")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}

	if err := json.NewEncoder(w).Encode(&updates); err != nil {
		services.Log.WithField("error", updates).Error("Error while trying to encode")
	}
}

func updateFromHTTP(w http.ResponseWriter, r *http.Request) *[]models.UpdateTransaction {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Info("Update is being created")
	account, orgID := readAccountOrOrgID(w, r, ctxServices.Log)
	if account == "" && orgID == "" {
		return nil
	}
	var devicesUpdate models.DevicesUpdate
	if err := readRequestJSONBody(w, r, ctxServices.Log, &devicesUpdate); err != nil {
		return nil
	}
	ctxServices.Log.WithField("updateJSON", devicesUpdate).Debug("Update JSON received")

	// TODO: Implement update by tag - Add validation per tag
	if devicesUpdate.DevicesUUID == nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("DeviceUUID required."))
		return nil
	}
	// remove any duplicates
	devicesUUID := make([]string, 0, len(devicesUpdate.DevicesUUID))
	devicesUUIDSMap := make(map[string]bool, len(devicesUpdate.DevicesUUID))
	for _, deviceUUID := range devicesUpdate.DevicesUUID {
		if _, ok := devicesUUIDSMap[deviceUUID]; !ok {
			devicesUUID = append(devicesUUID, deviceUUID)
			devicesUUIDSMap[deviceUUID] = true
		}
	}
	// check that all submitted devices exists
	var devicesCount int64
	if result := db.AccountOrOrg(account, orgID, "").Model(&models.Device{}).Where("uuid IN (?)", devicesUUID).Count(&devicesCount); result.Error != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error": result.Error.Error(),
		}).Error("failed to get devices count")
		apiError := errors.NewInternalServerError()
		apiError.SetTitle("failed to get devices count")
		respondWithAPIError(w, ctxServices.Log, apiError)
		return nil
	}
	if int64(len(devicesUUID)) != devicesCount {
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("some devices where not found"))
		return nil
	}
	if devicesUpdate.CommitID == 0 {
		commitID, err := ctxServices.DeviceService.GetLatestCommitFromDevices(account, orgID, devicesUUID)
		if err != nil {
			ctxServices.Log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("error when getting latest commit for devices")
			var apiError errors.APIError
			switch err.(type) {
			case *services.DeviceHasImageUndefined, *services.ImageHasNoImageSet, *services.DeviceHasMoreThanOneImageSet, *services.DeviceHasNoImageUpdate:
				apiError = errors.NewBadRequest(err.Error())
			default:
				apiError = errors.NewInternalServerError()
				apiError.SetTitle("failed to get latest commit for devices")
			}
			respondWithAPIError(w, ctxServices.Log, apiError)
			return nil
		}
		devicesUpdate.CommitID = commitID
	}
	//validate if commit is valid before continue process
	commit, err := ctxServices.CommitService.GetCommitByID(devicesUpdate.CommitID)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error":    err.Error(),
			"commitID": devicesUpdate.CommitID,
		}).Error("No commit found for Commit ID")
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound(fmt.Sprintf("No commit found for CommitID %d", devicesUpdate.CommitID)))
		return nil
	}
	ctxServices.Log.WithField("commit", commit.ID).Debug("Commit retrieved from this update")
	updates, err := ctxServices.UpdateService.BuildUpdateTransactions(&devicesUpdate, account, orgID, commit)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error":   err.Error(),
			"account": account,
			"org_id":  orgID,
		}).Error("Error building update transaction")
		apiError := errors.NewInternalServerError()
		apiError.SetTitle("Error building update transaction")
		respondWithAPIError(w, ctxServices.Log, apiError)
		return nil
	}
	return updates
}

// AddUpdate updates a device
func AddUpdate(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Info("Starting update")
	account, orgID := readAccountOrOrgID(w, r, ctxServices.Log)
	if account == "" && orgID == "" {
		return
	}
	updates := updateFromHTTP(w, r)
	if updates == nil {
		// errors handled by updateFromHTTP
		return
	}
	var upd []models.UpdateTransaction

	for _, update := range *updates {
		update.Account = account
		update.OrgID = orgID
		upd = append(upd, update)
		ctxServices.Log.WithField("updateID", update.ID).Info("Starting asynchronous update process")
		if update.Status != models.UpdateStatusDeviceDisconnected {
			ctxServices.UpdateService.CreateUpdateAsync(update.ID)
		}
	}
	if result := db.DB.Save(upd); result.Error != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error": result.Error.Error(),
		}).Error("Error saving update")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, updates)
}

// GetUpdateByID obtains an update from the database for an account
func GetUpdateByID(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		// Error set by UpdateCtx already
		return
	}
	if err := json.NewEncoder(w).Encode(update); err != nil {
		services := dependencies.ServicesFromContext(r.Context())
		services.Log.WithField("error", update).Error("Error while trying to encode")
	}
}

func getUpdate(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction {
	ctx := r.Context()
	update, ok := ctx.Value(UpdateContextKey).(*models.UpdateTransaction)
	if !ok {
		// Error set by UpdateCtx already
		return nil
	}
	return update
}

//SendNotificationForDevice TMP route to validate
func SendNotificationForDevice(w http.ResponseWriter, r *http.Request) {
	if update := getUpdate(w, r); update != nil {
		services := dependencies.ServicesFromContext(r.Context())
		notify, err := services.UpdateService.SendDeviceNotification(update)
		if err != nil {
			services.Log.WithField("error", err.Error()).Error("Failed to retry to send notification")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed creating image")
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
			return
		}
		services.Log.WithField("StatusOK", http.StatusOK).Info("Writting Header")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(notify); err != nil {
			services.Log.WithField("error", notify).Error("Error while trying to encode")
		}
	}
}

// ValidateUpdateResponse indicates whether or not the image can be updated
type ValidateUpdateResponse struct {
	UpdateValid bool `json:"UpdateValid"`
}

// PostValidateUpdate validate that images can be updated
func PostValidateUpdate(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	account, orgID := readAccountOrOrgID(w, r, services.Log)
	if account == "" && orgID == "" {
		return
	}

	var images []models.Image
	if err := readRequestJSONBody(w, r, services.Log, &images); err != nil {
		return
	}

	if len(images) == 0 {
		respondWithAPIError(w, services.Log, errors.NewBadRequest("It's expected at least one image"))
		return
	}

	ids := make([]uint, 0, len(images))
	for i := 0; i < len(images); i++ {
		ids = append(ids, images[i].ID)
	}

	valid, err := services.UpdateService.ValidateUpdateSelection(account, orgID, ids)

	if err != nil {
		services.Log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error validating the images selection")
		respondWithAPIError(w, services.Log, errors.NewBadRequest(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, services.Log, &ValidateUpdateResponse{UpdateValid: valid})
}
