// FIXME: golangci-lint
// nolint:govet,revive
package routes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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
	sub.With(ValidateQueryParams("updates")).With(common.Paginate).With(ValidateGetUpdatesFilterParams).Get("/", GetUpdates)
	sub.Post("/", AddUpdate)
	sub.Post("/validate", PostValidateUpdate)
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateCtx)
		r.Get("/", GetUpdateByID)
		r.Get("/update-playbook.yml", GetUpdatePlaybook)
		r.Get("/notify", SendNotificationForDevice) // TMP ROUTE TO SEND THE NOTIFICATION
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
		orgID := readOrgID(w, r, ctxServices.Log)
		if orgID == "" {
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
		if result := db.Org(orgID, "update_transactions").Preload("DispatchRecords").Preload("Devices").
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

// GetUpdatePlaybook returns the playbook for an update transaction
// @Summary      returns the playbook yaml file for a system update
// @ID           GetUpdatePlaybook
// @Description  returns the update transaction playbook used for system update
// @Tags         Updates (Systems)
// @Accept       json
// @Produce      plain
// @Param        updateID  path  integer    true  "a unique ID to identify the update the playbook belongs to" example(1042)
// @Success      200 {string}		"the playbook file content for an update"
// @Failure      400 {object} errors.BadRequest	"The request sent couldn't be processed."
// @Failure      404 {object} errors.NotFound	"the device update was not found"
// @Failure      500 {object} errors.InternalServerError	"There was an internal server error."
// @Router       /updates/{updateID}/update-playbook.yml [get]
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
		respondWithAPIError(w, services.Log, errors.NewNotFound("file was not found on the S3 bucket"))
		return
	}
	defer playbook.Close()
	_, err = io.Copy(w, playbook)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error reading the update playbook")
		respondWithAPIError(w, services.Log, errors.NewInternalServerError())
		return
	}
}

// GetUpdates returns the updates for the device
// @Summary      Gets all device updates
// @ID           ListUpdates
// @Description  Gets all device updates
// @Tags         Updates (Systems)
// @Accept       json
// @Produce      json
// @Param        limit query int false "field: return number of updates until limit is reached. Default is 30." example(20)
// @Param        offset query int false "field: return updates beginning at the given offset." example(30)
// @Param        sort_by query string false "fields: created_at, updated_at. To sort DESC use - before the fields." example(-created_at)
// @Param        status query string false "field: filter by status" example(BUILDING)
// @Param        created_at query string false "field: filter by creation date" example(2023-05-03)
// @Param        updated_at query string false "field: filter by update date" example(2023-05-04)
// @Success      200 {object} []models.UpdateAPI	"List of devices updates"
// @Failure      400 {object} errors.BadRequest	"The request sent couldn't be processed."
// @Failure      500 {object} errors.InternalServerError	"There was an internal server error."
// @Router       /updates [get]
func GetUpdates(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	result := updateFilters(r, db.DB)
	pagination := common.GetPagination(r)
	var updates []models.UpdateTransaction
	orgID := readOrgID(w, r, services.Log)
	if orgID == "" {
		return
	}
	if result = db.OrgDB(orgID, result, "update_transactions").Limit(pagination.Limit).Offset(pagination.Offset).Preload("DispatchRecords").Preload("Devices").
		Joins("Commit").Joins("Repo").Find(&updates); result.Error != nil {
		services.Log.WithFields(log.Fields{
			"error": result.Error.Error(),
		}).Error("Error retrieving updates")
		respondWithAPIError(w, services.Log, errors.NewInternalServerError())
		return
	}

	respondWithJSONBody(w, services.Log, &updates)
}

func updateFromHTTP(w http.ResponseWriter, r *http.Request) *[]models.UpdateTransaction {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Info("Update is being created")
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
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
	if result := db.Org(orgID, "").Model(&models.Device{}).Where("uuid IN (?)", devicesUUID).Count(&devicesCount); result.Error != nil {
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

	// get the latest commit for devices
	var commit *models.Commit
	if devicesUpdate.CommitID == 0 {
		commitID, err := ctxServices.DeviceService.GetLatestCommitFromDevices(orgID, devicesUUID)
		if err != nil {
			ctxServices.Log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("error when getting latest commit for devices")
			var apiError errors.APIError
			switch err.(type) {
			case *services.DeviceHasImageUndefined, *services.ImageHasNoImageSet, *services.DevicesHasMoreThanOneImageSet, *services.DeviceHasNoImageUpdate:
				apiError = errors.NewBadRequest(err.Error())
			default:
				apiError = errors.NewInternalServerError()
				apiError.SetTitle("failed to get latest commit for devices")
			}
			respondWithAPIError(w, ctxServices.Log, apiError)
			return nil
		}
		devicesUpdate.CommitID = commitID
		commit, err = ctxServices.CommitService.GetCommitByID(devicesUpdate.CommitID, orgID)
		if err != nil {
			ctxServices.Log.WithFields(log.Fields{
				"error":    err.Error(),
				"commitID": devicesUpdate.CommitID,
			}).Error("Can't find latest commit for the devices")
			respondWithAPIError(w, ctxServices.Log, errors.NewNotFound(fmt.Sprintf("No commit found for CommitID %d", devicesUpdate.CommitID)))
			return nil
		}
	} else {
		// validate if commit is valid before continue process
		var err error
		commit, err = ctxServices.CommitService.GetCommitByID(devicesUpdate.CommitID, orgID)
		if err != nil {
			ctxServices.Log.WithFields(log.Fields{
				"error":    err.Error(),
				"commitID": devicesUpdate.CommitID,
			}).Error("No commit found for Commit ID")
			respondWithAPIError(w, ctxServices.Log, errors.NewNotFound(fmt.Sprintf("No commit found for CommitID %d", devicesUpdate.CommitID)))
			return nil
		}
		// validate if user provided commitID belong to same ImageSet as of Device Image
		if err := ctxServices.CommitService.ValidateDevicesImageSetWithCommit(devicesUUID, devicesUpdate.CommitID); err != nil {
			ctxServices.Log.WithFields(log.Fields{
				"error":    err.Error(),
				"commitID": devicesUpdate.CommitID,
			}).Error(err.Error())
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(fmt.Sprintf("Commit %d %v", devicesUpdate.CommitID, err.Error())))
			return nil
		}
	}
	ctxServices.Log.WithField("commit", commit.ID).Debug("Commit retrieved from this update")
	updates, err := ctxServices.UpdateService.BuildUpdateTransactions(&devicesUpdate, orgID, commit)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error":  err.Error(),
			"org_id": orgID,
		}).Error("Error building update transaction")
		apiError := errors.NewInternalServerError()
		apiError.SetTitle("Error building update transaction")
		respondWithAPIError(w, ctxServices.Log, apiError)
		return nil
	}

	if len(*updates) == 0 {
		ctxServices.Log.WithFields(log.Fields{
			"org_id": orgID,
		}).Info("There are no updates to perform")
		respondWithJSONBody(w, ctxServices.Log, common.APIResponse{Message: "There are no updates to perform"})
		return nil
	}

	return updates
}

// AddUpdate updates a device
// @Summary      Executes a device update
// @ID           UpdateDevice
// @Description  Executes a device update
// @Tags         Updates (Systems)
// @Accept       json
// @Produce      json
// @Param        body	body	models.DevicesUpdateAPI	true	"devices uuids to update and optional target commit id"
// @Success      200 {object} models.UpdateAPI	"The created device update"
// @Failure      400 {object} errors.BadRequest	"The request sent couldn't be processed"
// @Failure      500 {object} errors.InternalServerError	"There was an internal server error"
// @Router       /updates [post]
func AddUpdate(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctxServices.Log.Info("Starting update")
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	updates := updateFromHTTP(w, r)
	if updates == nil {
		// errors handled by updateFromHTTP
		return
	}
	var upd []models.UpdateTransaction

	for _, update := range *updates {
		update.OrgID = orgID
		upd = append(upd, update)
		ctxServices.Log.WithField("updateID", update.ID).Info("Starting asynchronous update process")
		if update.Status != models.UpdateStatusDeviceDisconnected {
			ctxServices.UpdateService.CreateUpdateAsync(update.ID)
		}
	}
	if result := db.DB.Omit("Devices.*").Save(upd); result.Error != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error": result.Error.Error(),
		}).Error("Error saving update")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, updates)
}

// GetUpdateByID obtains an update from the database for an orgID
// @Summary      Gets a single requested update
// @ID           GetUpdate
// @Description  Gets a single requested update.
// @Tags         Updates (Systems)
// @Accept       json
// @Produce      json
// @Param        updateID  path  integer    true  "a unique ID to identify the update" example(1042)
// @Success      200 {object} models.UpdateAPI	"The requested update"
// @Failure      400 {object} errors.BadRequest	"The request sent couldn't be processed"
// @Failure      404 {object} errors.NotFound	"The requested update was not found"
// @Failure      500 {object} errors.InternalServerError	"There was an internal server error"
// @Router       /updates/{updateID} [get]
func GetUpdateByID(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	update := getUpdate(w, r)
	if update == nil {
		// Error set by UpdateCtx already
		return
	}
	respondWithJSONBody(w, ctxServices.Log, update)
}

func getUpdate(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	ctx := r.Context()
	update, ok := ctx.Value(UpdateContextKey).(*models.UpdateTransaction)
	if !ok {
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("update-transaction not found in context"))
		return nil
	}
	return update
}

// SendNotificationForDevice TMP route to validate
// @Summary      Send a notification for a device update
// @ID           SendNotificationForDevice
// @Description  Send a notification for a device update
// @Tags         Updates (Systems)
// @Accept       json
// @Produce      json
// @Param        updateID  path  integer    true  "a unique ID to identify the update" example(1042)
// @Success      200 {object} models.DeviceNotificationAPI "The notification payload"
// @Failure      400 {object} errors.BadRequest	"The request sent couldn't be processed"
// @Failure      404 {object} errors.NotFound	"The requested update was not found"
// @Failure      500 {object} errors.InternalServerError	"There was an internal server error"
// @Router       /updates/{updateID}/notify [get]
func SendNotificationForDevice(w http.ResponseWriter, r *http.Request) {
	if update := getUpdate(w, r); update != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		notify, err := ctxServices.UpdateService.SendDeviceNotification(update)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Failed to retry to send notification")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed to send notification")
			respondWithAPIError(w, ctxServices.Log, err)
			return
		}
		ctxServices.Log.WithField("StatusOK", http.StatusOK).Info("Writing Header")

		w.WriteHeader(http.StatusOK)
		respondWithJSONBody(w, ctxServices.Log, &notify)
	}
}

// ValidateUpdateResponse indicates whether or not the image can be updated
type ValidateUpdateResponse struct {
	UpdateValid bool `json:"UpdateValid"`
}

// PostValidateUpdate validate that images can be updated
// @Summary      Validate if the images selection could be updated
// @ID           PostValidateUpdate
// @Description  Validate if the images selection could be updated
// @Tags         Updates (Systems)
// @Accept       json
// @Produce      json
// @Param        body	body	[]models.ImageValidationRequestAPI	true	"request body"
// @Success      200 {object} models.ImageValidationResponseAPI	"the validation result"
// @Failure      400 {object} errors.BadRequest	"The request sent couldn't be processed"
// @Failure      500 {object} errors.InternalServerError	"There was an internal server error"
// @Router       /updates/validate [post]
func PostValidateUpdate(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, services.Log)
	if orgID == "" {
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

	valid, err := services.UpdateService.ValidateUpdateSelection(orgID, ids)

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

var updateFilters = common.ComposeFilters(
	// Filter handler for "status"
	common.OneOfFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "update_transactions.status",
	}),
	// Filter handler for "created_at"
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "update_transactions.created_at",
	}),
	common.SortFilterHandler("update_transactions", "created_at", "DESC"),
)

// ValidateGetUpdatesFilterParams validate the query params that sent to /updates endpoint
func ValidateGetUpdatesFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		var errs []validationError

		// "created_at" validation
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()})
			}
		}
		// "updated_at" validation
		if val := r.URL.Query().Get("updated_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "updated_at", Reason: err.Error()})
			}
		}
		// "sort_by" validation for "name", "created_at", "updated_at"
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" {
				name = val[1:]
			}
			if name != "created_at" && name != "updated_at" {
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be created_at or updated_at", name)})
			}
		}

		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		respondWithJSONBody(w, ctxServices.Log, &errs)
	})
}
