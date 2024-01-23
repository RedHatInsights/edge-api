// FIXME: golangci-lint
// nolint:gocritic,govet,revive,staticcheck,typecheck
package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

type deviceGroupTypeKey string

const deviceGroupKey = deviceGroupTypeKey("device_group_key")

func setContextDeviceGroup(ctx context.Context, deviceGroup *models.DeviceGroup) context.Context {
	return context.WithValue(ctx, deviceGroupKey, deviceGroup)
}

func setContextDeviceGroupDetails(ctx context.Context, deviceGroup *models.DeviceGroupDetails) context.Context {
	return context.WithValue(ctx, deviceGroupKey, deviceGroup)
}

type deviceGroupDeviceKeyType string

const deviceGroupDeviceKey = deviceGroupDeviceKeyType("device_group_device_key")

func setContextDeviceGroupDevice(ctx context.Context, deviceGroupDevice *models.Device) context.Context {
	return context.WithValue(ctx, deviceGroupDeviceKey, deviceGroupDevice)
}

// MakeDeviceGroupsRouter adds support for device groups operations
func MakeDeviceGroupsRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("device-groups")).With(ValidateGetAllDeviceGroupsFilterParams).With(common.Paginate).Get("/", GetAllDeviceGroups)
	sub.Post("/", CreateDeviceGroup)
	sub.Get("/checkName/{name}", CheckGroupName)
	sub.Get("/enforce-edge-groups", GetEnforceEdgeGroups)
	sub.Route("/{ID}", func(r chi.Router) {
		r.Use(DeviceGroupCtx)
		r.Get("/", GetDeviceGroupByID)
		r.Put("/", UpdateDeviceGroup)
		r.Delete("/", DeleteDeviceGroupByID)
		r.Post("/devices", AddDeviceGroupDevices)
		r.Delete("/devices", DeleteDeviceGroupManyDevices)
		r.Route("/details", func(d chi.Router) {
			d.Use(DeviceGroupDetailsCtx)
			d.Get("/", GetDeviceGroupDetailsByID)
		})
		r.Route("/view", func(d chi.Router) {
			d.With(common.Paginate).Get("/", GetDeviceGroupDetailsByIDView)
		})
		r.Route("/devices/{DEVICE_ID}", func(d chi.Router) {
			d.Use(DeviceGroupDeviceCtx)
			d.Delete("/", DeleteDeviceGroupOneDevice)
		})

		r.Post("/updateDevices", UpdateAllDevicesFromGroup)

	})
}

// DeviceGroupDetailsCtx is a handler to Device Group Details requests
func DeviceGroupDetailsCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		if ID := chi.URLParam(r, "ID"); ID != "" {
			_, err := strconv.Atoi(ID)
			ctxServices.Log = ctxServices.Log.WithField("deviceGroupID", ID)
			ctxServices.Log.Debug("Retrieving device group")
			if err != nil {
				ctxServices.Log.Debug("ID is not an integer")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
				return
			}

			deviceGroup, err := ctxServices.DeviceGroupsService.GetDeviceGroupDetailsByID(ID)
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.DeviceGroupNotFound:
					responseErr = errors.NewNotFound(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
					responseErr.SetTitle("failed getting device group")
				}
				respondWithAPIError(w, ctxServices.Log, responseErr)
				return
			}
			orgID := readOrgID(w, r, ctxServices.Log)
			if orgID == "" {
				// logs and response handled by readOrgID
				return
			}
			ctx := setContextDeviceGroupDetails(r.Context(), deviceGroup)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			ctxServices.Log.Debug("deviceGroup ID was not passed to the request or it was empty")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("deviceGroup ID required"))
			return
		}
	})
}

// DeviceGroupCtx is a handler to Device Group requests
func DeviceGroupCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		if ID := chi.URLParam(r, "ID"); ID != "" {
			_, err := strconv.Atoi(ID)
			ctxServices.Log = ctxServices.Log.WithField("deviceGroupID", ID)
			ctxServices.Log.Debug("Retrieving device group")
			if err != nil {
				ctxServices.Log.Debug("ID is not an integer")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
				return
			}

			deviceGroup, err := ctxServices.DeviceGroupsService.GetDeviceGroupByID(ID)
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.DeviceGroupNotFound:
					responseErr = errors.NewNotFound(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
					responseErr.SetTitle("failed getting device group")
				}
				respondWithAPIError(w, ctxServices.Log, responseErr)
				return
			}
			orgID := readOrgID(w, r, ctxServices.Log)
			if orgID == "" {
				// logs and response handled by readOrgID
				return
			}
			ctx := setContextDeviceGroup(r.Context(), deviceGroup)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			ctxServices.Log.Debug("deviceGroup ID was not passed to the request or it was empty")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("deviceGroup ID required"))
			return
		}
	})
}

// DeviceGroupDeviceCtx is a handler to DeviceGroup Device requests
func DeviceGroupDeviceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		deviceGroup := getContextDeviceGroup(w, r)
		if deviceGroup == nil {
			ctxServices.Log.Debug("device-group not defined")
			return
		}
		if strDeviceID := chi.URLParam(r, "DEVICE_ID"); strDeviceID != "" {
			deviceID, err := strconv.ParseUint(strDeviceID, 10, 32)
			deviceLog := ctxServices.Log.WithField("deviceID", strDeviceID)
			deviceLog.Debug("Retrieving device-group device")
			if err != nil {
				deviceLog.Debug("deviceID is not an integer")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
				return
			}
			orgID := readOrgID(w, r, ctxServices.Log)
			if orgID == "" {
				// logs and response handled by readOrgID
				return
			}
			deviceGroupDevice, err := ctxServices.DeviceGroupsService.GetDeviceGroupDeviceByID(orgID, deviceGroup.ID, uint(deviceID))
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.DeviceGroupNotFound:
					responseErr = errors.NewNotFound(err.Error())
				case *services.DeviceGroupDeviceNotSupplied:
					responseErr = errors.NewBadRequest("Device group device not supplied")
				default:
					responseErr = errors.NewInternalServerError()
					responseErr.SetTitle("failed getting device group")
				}
				respondWithAPIError(w, ctxServices.Log, responseErr)
				return
			}

			ctx := setContextDeviceGroupDevice(r.Context(), deviceGroupDevice)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			ctxServices.Log.Debug("deviceGroup deviceID was not passed to the request or it was empty")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("deviceGroup deviceID required"))
			return
		}
	})
}

var deviceGroupsFilters = common.ComposeFilters(
	// Filter handler for "name"
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "device_groups.name",
	}),
	// Filter handler for "created_at"
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "device_groups.created_at",
	}),
	// Filter handler for "updated_at"
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "updated_at",
		DBField:    "device_groups.updated_at",
	}),
	// Filter handler for sorting "created_at"
	common.SortFilterHandler("device_groups", "created_at", "DESC"),
)

// ValidateGetAllDeviceGroupsFilterParams validate the query params that sent to /device-groups endpoint
func ValidateGetAllDeviceGroupsFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			if name != "name" && name != "created_at" && name != "updated_at" {
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be name or created_at or updated_at", name)})
			}
		}

		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(&errs); err != nil {
			ctxServices := dependencies.ServicesFromContext(r.Context())
			ctxServices.Log.WithField("error", errs).Error("Error while trying to encode device groups filter validation errors")
		}
	})
}

// GetAllDeviceGroups Returns device groups for an orgID
// @Summary      Returns device groups for an orgID
// @Description  Returns device groups for an orgID
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param        sort_by    query     string     false  "Define sort fields: created_at, updated_at, name. To sort DESC use -"
// @Param        name       query     string     false  "field: filter by name"
// @Param        limit      query     int        false  "field: return number of image-set view until limit is reached. Default is 100."
// @Param        offset     query     int        false  "field: return number of image-set view beginning at the offset."
// @Success      200 {object} models.DeviceGroup
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups [get]
func GetAllDeviceGroups(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	deviceGroupService := ctxServices.DeviceGroupsService
	tx := deviceGroupsFilters(r, db.DB)

	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	pagination := common.GetPagination(r)

	deviceGroupsCount, err := deviceGroupService.GetDeviceGroupsCount(orgID, tx)
	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	deviceGroups, err := deviceGroupService.GetDeviceGroups(orgID, pagination.Limit, pagination.Offset, tx)
	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": deviceGroups, "count": deviceGroupsCount})
}

// CreateDeviceGroup is the route to create a new device group
// @Summary      Creates a Device Group for an account.
// @ID 			 CreateDeviceGroup
// @Description  Creates a Device Group for an account.
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param        body	body models.CreateDeviceGroupAPI	true	"request body"
// @Success      200 {object} models.DeviceGroupAPI "The created device groups"
// @Failure      400 {object} errors.BadRequest "The request sent couldn't be processed."
// @Failure      500 {object} errors.InternalServerError "There was an internal server error."
// @Router       /device-groups [post]
func CreateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}

	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	if feature.HideCreateGroup.IsEnabled() {
		w.WriteHeader(http.StatusUnauthorized)
		respondWithJSONBody(w, ctxServices.Log, nil)
		return
	} else {
		deviceGroup, err := createDeviceRequest(w, r)
		if err != nil {
			return
		}
		ctxServices.Log.Debug("Creating a device group")

		deviceGroup, err = ctxServices.DeviceGroupsService.CreateDeviceGroup(deviceGroup)
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Error creating a device group")
			var apiError errors.APIError
			switch err.(type) {
			case *services.DeviceGroupAlreadyExists:
				apiError = errors.NewBadRequest(err.Error())
			default:
				apiError := errors.NewInternalServerError()
				apiError.SetTitle("failed updating device group")
			}
			respondWithAPIError(w, ctxServices.Log, apiError)
			return
		}

		w.WriteHeader(http.StatusOK)
		respondWithJSONBody(w, ctxServices.Log, &deviceGroup)
	}
}

// GetDeviceGroupDetailsByID Returns details for group identified by ID
// @Summary      Returns details for group identified by ID
// @Description  Returns details for group identified by ID
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 required_param query int true "device group ID" example(123)
// @Success      200 {object} models.DeviceGroup
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID}/details [get]
func GetDeviceGroupDetailsByID(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	if deviceGroup := getContextDeviceGroupDetails(w, r); deviceGroup != nil {
		respondWithJSONBody(w, ctxServices.Log, deviceGroup)
	}
}

// GetDeviceGroupDetailsByIDView Returns devices groups view for group identified by ID
// @Summary      Returns device groups view for group identified by ID
// @Description  Returns device groups view for group identified by ID
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 required_param query int false "device group ID"
// @Success      200 {object} models.DeviceGroupViewAPI
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID}/view [get]
func GetDeviceGroupDetailsByIDView(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}

	deviceGroup := getContextDeviceGroup(w, r)
	if deviceGroup == nil {
		return
	}

	var deviceGroupDetails models.DeviceGroupDetailsView
	deviceGroupDetails.DeviceGroup = deviceGroup
	deviceGroupDetails.DeviceDetails.EnforceEdgeGroups = utility.EnforceEdgeGroups(orgID)
	if int(len(deviceGroup.Devices)) == 0 {
		respondWithJSONBody(w, ctxServices.Log, &deviceGroupDetails)
		return
	}

	devicesIDS := make([]uint, 0, len(deviceGroup.Devices))
	for _, device := range deviceGroup.Devices {
		devicesIDS = append(devicesIDS, device.ID)
	}

	tx := devicesFilters(r, db.DB).
		Where("image_id IS NOT NULL AND image_id != 0 AND ID IN (?)", devicesIDS)

	devicesCount, err := ctxServices.DeviceService.GetDevicesCount(tx)
	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("No devices found"))
		return
	}

	pagination := common.GetPagination(r)
	deviceGroupDevices, err := ctxServices.DeviceService.GetDevicesView(pagination.Limit, pagination.Offset, tx)

	if err != nil {
		var responseErr errors.APIError
		switch err.(type) {
		case *services.DeviceGroupNotFound:
			responseErr = errors.NewNotFound(err.Error())
		default:
			responseErr = errors.NewInternalServerError()
			responseErr.SetTitle("failed getting device group")
		}
		respondWithAPIError(w, ctxServices.Log, responseErr)
		return
	}

	deviceGroupDetails.DeviceDetails.Devices = deviceGroupDevices.Devices
	deviceGroupDetails.DeviceDetails.Total = devicesCount

	respondWithJSONBody(w, ctxServices.Log, &deviceGroupDetails)
}

// GetDeviceGroupByID Returns device groups for group identified by ID
// @Summary      Returns devices groups for group identified by ID
// @Description  Returns devices groups for group identified by ID
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 required_param query int false "device group ID"
// @Success      200 {object} models.DeviceGroup
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID} [get]
func GetDeviceGroupByID(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	if deviceGroup := getContextDeviceGroup(w, r); deviceGroup != nil {
		respondWithJSONBody(w, ctxServices.Log, deviceGroup)
	}
}

func getContextDeviceGroupDetails(w http.ResponseWriter, r *http.Request) *models.DeviceGroupDetails {
	ctx := r.Context()
	deviceGroupDetails, ok := ctx.Value(deviceGroupKey).(*models.DeviceGroupDetails)

	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting device group from context"))
		return nil
	}
	return deviceGroupDetails
}

func getContextDeviceGroup(w http.ResponseWriter, r *http.Request) *models.DeviceGroup {
	ctx := r.Context()
	deviceGroup, ok := ctx.Value(deviceGroupKey).(*models.DeviceGroup)

	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting device group from context"))
		return nil
	}
	return deviceGroup
}

// UpdateDeviceGroup Updates the existing device group
// @Summary      Updates the existing device group
// @Description  Updates the existing device group
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param        required_param query int true "An unique existing Device Group" example(1080)
// @Param        body	body	models.PutGroupNameParamAPI	true	"request body"
// @Success      200 {object} models.DeviceGroup
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID} [put]
func UpdateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	if oldDeviceGroup := getContextDeviceGroup(w, r); oldDeviceGroup != nil {
		deviceGroup, err := createDeviceRequest(w, r)
		if err != nil {
			// error handled by createRequest already
			return
		}
		orgID := readOrgID(w, r, ctxServices.Log)
		if orgID == "" {
			// logs and response handled by readOrgID
			return
		}
		err = ctxServices.DeviceGroupsService.UpdateDeviceGroup(deviceGroup, orgID, fmt.Sprint(oldDeviceGroup.ID))
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Error updating device group")
			var apiError errors.APIError
			switch err.(type) {
			case *services.DeviceGroupAlreadyExists:
				apiError = errors.NewBadRequest(err.Error())
			default:
				apiError = errors.NewInternalServerError()
				apiError.SetTitle("failed updating device group")
			}
			respondWithAPIError(w, ctxServices.Log, apiError)
			return
		}
		w.WriteHeader(http.StatusOK)
		updatedDeviceGroup, err := ctxServices.DeviceGroupsService.GetDeviceGroupByID(fmt.Sprint(oldDeviceGroup.ID))
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Error getting device group")
			err := errors.NewInternalServerError()
			err.SetTitle("failed to get device group")
			respondWithAPIError(w, ctxServices.Log, err)
		} else {
			respondWithJSONBody(w, ctxServices.Log, updatedDeviceGroup)
		}
	}
}

// DeleteDeviceGroupByID Deletes an existing device group
// @Summary      Deletes an existing device group
// @Description  Deletes an existing device group
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 required_param query int true "A unique existing Device Group" example(1080)
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID} [delete]
func DeleteDeviceGroupByID(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	deviceGroup := getContextDeviceGroup(w, r)
	if deviceGroup == nil {
		return // error handled by getContextDeviceGroup already
	}
	ctxLog := ctxServices.Log.WithField("device_group_id", deviceGroup.ID)
	ctxLog.Debug("Deleting a device group")
	err := ctxServices.DeviceGroupsService.DeleteDeviceGroupByID(fmt.Sprint(deviceGroup.ID))
	if err != nil {
		ctxLog.WithField("error", err.Error()).Error("Error deleting device group")
		var apiError errors.APIError
		switch err.(type) {
		case *services.AccountNotSet:
			apiError = errors.NewBadRequest(err.Error())
		case *services.OrgIDNotSet:
			apiError = errors.NewBadRequest(err.Error())
		case *services.DeviceGroupNotFound:
			apiError = errors.NewNotFound(err.Error())
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxLog, apiError)
		return
	}
	respondWithJSONBody(w, ctxLog, map[string]interface{}{"message": "Device group deleted"})
}

// createDeviceRequest validates request to create Device Group.
func createDeviceRequest(w http.ResponseWriter, r *http.Request) (*models.DeviceGroup, error) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	var deviceGroup *models.DeviceGroup
	if err := readRequestJSONBody(w, r, ctxServices.Log, &deviceGroup); err != nil {
		return nil, err
	}

	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return nil, errors.NewBadRequest("could not read org id")
	}

	ctxServices.Log = ctxServices.Log.WithFields(log.Fields{
		"name":    deviceGroup.Name,
		"account": deviceGroup.Account,
		"orgID":   deviceGroup.OrgID,
	})
	deviceGroup.OrgID = orgID

	if err := deviceGroup.ValidateRequest(); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error validation request from device group")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return nil, err
	}
	return deviceGroup, nil
}

func getContextDeviceGroupDevice(w http.ResponseWriter, r *http.Request) *models.Device {
	ctx := r.Context()
	deviceGroupDevice, ok := ctx.Value(deviceGroupDeviceKey).(*models.Device)
	if !ok {
		ctxServices := dependencies.ServicesFromContext(ctx)
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting device-group device from context"))
		return nil
	}
	return deviceGroupDevice
}

// AddDeviceGroupDevices Adds devices to device group
// @Summary      Adds devices to device group
// @Description  Adds devices to device group
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param        required_param query int true "An unique existing Device Group" example(1080)
// @Param        required_param body	body	models.PostDeviceForDeviceGroupAPI	true	"request body"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID}/devices [post]
func AddDeviceGroupDevices(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}

	contextDeviceGroup := getContextDeviceGroup(w, r)
	if contextDeviceGroup == nil {
		return
	}

	var requestDeviceGroup models.DeviceGroup
	if err := readRequestJSONBody(w, r, ctxServices.Log, &requestDeviceGroup); err != nil {
		return
	}
	devicesAdded, err := ctxServices.DeviceGroupsService.AddDeviceGroupDevices(orgID, contextDeviceGroup.ID, requestDeviceGroup.Devices)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error when adding deviceGroup devices")
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceGroupDevicesNotSupplied, *services.DeviceGroupMandatoryFieldsUndefined:
			apiError = errors.NewBadRequest(err.Error())
		case *services.DeviceGroupOrgIDDevicesNotFound:
			apiError = errors.NewNotFound(err.Error())
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	respondWithJSONBody(w, ctxServices.Log, devicesAdded)
}

// DeleteDeviceGroupManyDevices Deletes the requested devices from device-group
// @Summary      Deletes the requested devices from device-group
// @Description  Deletes the requested devices from device-group
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 DeviceGroupID		path    int  true  "Identifier of the DeviceGroup"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID}/devices [delete]
func DeleteDeviceGroupManyDevices(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	contextDeviceGroup := getContextDeviceGroup(w, r)
	if contextDeviceGroup == nil {
		return
	}

	var requestDeviceGroup models.DeviceGroup
	if err := readRequestJSONBody(w, r, ctxServices.Log, &requestDeviceGroup); err != nil {
		return
	}

	deletedDevices, err := ctxServices.DeviceGroupsService.DeleteDeviceGroupDevices(orgID, contextDeviceGroup.ID, requestDeviceGroup.Devices)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error when removing deviceGroup devices")
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceGroupDevicesNotSupplied, *services.DeviceGroupMandatoryFieldsUndefined:
			apiError = errors.NewBadRequest(err.Error())
		case *services.DeviceGroupDevicesNotFound:
			apiError = errors.NewNotFound(err.Error())
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	respondWithJSONBody(w, ctxServices.Log, deletedDevices)
}

// DeleteDeviceGroupOneDevice Deletes the requested device from the device-group
// @Summary      Deletes the requested device from the device-group
// @Description  Deletes the requested device from the device-group
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 DeviceGroupId		path    int  true  "Identifier of the Device Group"
// @Param		 DeviceId		path    int  true  "Identifier of the Device in a Device Group"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID}/devices/{deviceID} [delete]
func DeleteDeviceGroupOneDevice(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	contextDeviceGroup := getContextDeviceGroup(w, r)
	contextDeviceGroupDevice := getContextDeviceGroupDevice(w, r)
	if contextDeviceGroupDevice == nil {
		return
	}

	_, err := ctxServices.DeviceGroupsService.DeleteDeviceGroupDevices(
		orgID, contextDeviceGroup.ID, []models.Device{*contextDeviceGroupDevice},
	)

	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error when removing deviceGroup devices")
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceGroupDevicesNotSupplied, *services.DeviceGroupMandatoryFieldsUndefined:
			apiError = errors.NewBadRequest(err.Error())
		case *services.DeviceGroupDevicesNotFound:
			apiError = errors.NewNotFound(err.Error())
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	respondWithJSONBody(w, ctxServices.Log, contextDeviceGroupDevice)
}

// CheckGroupName Validates if a group name already exists
// @Summary      Validates if a group name already exists
// @Description  Validates if a group name already exists
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 body	body models.CheckGroupNameParamAPI true	"request body"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/checkName/{name} [get]
func CheckGroupName(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	var name = chi.URLParam(r, "name")

	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	value, err := ctxServices.DeviceGroupsService.DeviceGroupNameExists(orgID, name)

	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
	}

	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": map[string]interface{}{"isValid": value}})
}

// UpdateAllDevicesFromGroup Updates all devices that belong to a group
// @Summary      Updates all devices that belong to a group
// @Description  Updates all devices that belong to a group
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Param		 required_param query int  true  "Identifier of the DeviceGroup"
// @Success      200 {object} models.SuccessPlaceholderResponse
// @Failure      400 {object} errors.BadRequest
// @Failure      500 {object} errors.InternalServerError
// @Router       /device-groups/{ID}/updateDevices [post]
func UpdateAllDevicesFromGroup(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		return
	}
	if feature.EdgeParityInventoryGroupsEnabled.IsEnabled() && !utility.EnforceEdgeGroups(orgID) {
		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable(""))
		return
	}
	deviceGroup := getContextDeviceGroup(w, r)
	if deviceGroup == nil {
		return
	}
	ctxLog := ctxServices.Log.WithField("device_group_id", deviceGroup.ID)
	ctxLog.Info("Updating all devices from group", deviceGroup.ID)

	var setOfImageSetID []uint
	var setOfDeviceUUIDS []string

	for _, d := range deviceGroup.Devices {
		var img models.Image
		err := db.DB.Joins("Images").Find(&img,
			"id = ?", d.ImageID)
		if err.Error != nil {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(fmt.Sprintf(err.Error.Error())))
			return
		}
		if setOfImageSetID != nil && !containsInt(setOfImageSetID, *img.ImageSetID) {
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("can't update devices with different image set ID"))
			return
		}
		setOfImageSetID = append(setOfImageSetID, *img.ImageSetID)
		setOfDeviceUUIDS = append(setOfDeviceUUIDS, d.UUID)

	}

	var devicesUpdate models.DevicesUpdate
	devicesUpdate.DevicesUUID = setOfDeviceUUIDS
	// validate if commit is valid before continue process
	// should be created a new method to return the latest commit by imageId and be able to update regardless of imageset
	commitID, err := ctxServices.DeviceService.GetLatestCommitFromDevices(orgID, setOfDeviceUUIDS)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error":  err.Error(),
			"org_id": orgID,
		}).Error("Error Getting the latest commit to update a device")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	devicesUpdate.CommitID = commitID
	// get commit info to build update repo
	commit, err := ctxServices.CommitService.GetCommitByID(devicesUpdate.CommitID, orgID)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error":  err.Error(),
			"org_id": orgID,
		}).Error("Error Getting the commit info to update a device")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	// should be refactored to avoid performance issue with large volume
	updates, err := ctxServices.UpdateService.BuildUpdateTransactions(&devicesUpdate, orgID, commit)
	if err != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error":  err.Error(),
			"org_id": orgID,
		}).Error("Error building update transaction")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	// should be refactored to avoid performance issue with large volume
	var upd []models.UpdateTransaction
	for _, update := range *updates {
		update.OrgID = orgID
		upd = append(upd, update)
		ctxServices.Log.WithField("updateID", update.ID).Debug("Starting asynchronous update process")
		ctxServices.UpdateService.CreateUpdateAsync(update.ID)
	}
	if len(upd) == 0 {
		respondWithAPIError(w, ctxServices.Log, errors.NewNotFound("devices not found"))
		return
	}
	result := db.DB.Omit("Devices").Save(upd)
	if result.Error != nil {
		ctxServices.Log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error saving update")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, updates)
}

func containsInt(s []uint, searchterm uint) bool {
	for _, a := range s {
		if a == searchterm {
			return true
		}
	}
	return false
}

// GetEnforceEdgeGroups Returns whether the edge groups is enforced for the current organization
// @Summary      Returns whether the edge groups is enforced for the current organization
// @Description  Returns whether the edge groups is enforced for the current organization
// @Tags         Device Groups
// @Accept       json
// @Produce      json
// @Success      200 {object} models.EnforceEdgeGroupsAPI
// @Failure      400 {object} errors.BadRequest
// @Router       /device-groups/enforce-edge-groups [get]
func GetEnforceEdgeGroups(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, &models.EnforceEdgeGroupsAPI{EnforceEdgeGroups: utility.EnforceEdgeGroups(orgID)})
}
