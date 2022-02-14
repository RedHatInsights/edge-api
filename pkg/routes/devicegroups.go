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

// MakeDeviceGroupsRouter adds support for device groups operations
func MakeDeviceGroupsRouter(sub chi.Router) {
	sub.With(validateGetAllDeviceGroupsFilterParams).With(common.Paginate).Get("/", GetAllDeviceGroups)
	sub.Post("/", CreateDeviceGroup)
	sub.Route("/{ID}", func(r chi.Router) {
		r.Use(DeviceGroupCtx)
		r.Get("/", GetDeviceGroupByID)
		r.Put("/", UpdateDeviceGroup)
		r.Delete("/", DeleteDeviceGroupByID)
		r.Post("/devices", AddDeviceGroupDevices)
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
					responseErr.SetTitle("failed getting third device group")
				}
				respondWithAPIError(w, ctxServices.Log, responseErr)
				return
			}
			account, err := common.GetAccount(r)
			if err != nil || deviceGroup.Account != account {
				ctxServices.Log.WithFields(log.Fields{
					"error":   err.Error(),
					"account": account,
				}).Error("Error retrieving account or device group doesn't belong to account")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
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

func respondWithAPIError(w http.ResponseWriter, log *log.Entry, apiError errors.APIError) {
	w.WriteHeader(apiError.GetStatus())
	if err := json.NewEncoder(w).Encode(&apiError); err != nil {
		log.WithField("error", err.Error()).Error("Error while trying to encode api error")
	}
}

func respondWithJSONBody(w http.ResponseWriter, log *log.Entry, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.WithField("error", data).Error("Error while trying to encode")
		respondWithAPIError(w, log, errors.NewInternalServerError())
	}
}

func readRequestJSONBody(w http.ResponseWriter, r *http.Request, log *log.Entry, dataReceiver interface{}) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dataReceiver); err != nil {
		log.WithField("error", err.Error()).Error("Error parsing json from device group request")
		respondWithAPIError(w, log, errors.NewBadRequest("invalid JSON request"))
		return err
	}
	return nil
}

var deviceGroupsFilters = common.ComposeFilters(
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "device_groups.name",
	}),
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "device_groups.created_at",
	}),
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "updated_at",
		DBField:    "device_groups.updated_at",
	}),
	common.SortFilterHandler("device_groups", "created_at", "DESC"),
)

func validateGetAllDeviceGroupsFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var errs []validationError
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()})
			}
		}
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

// GetAllDeviceGroups return devices groups for an account
func GetAllDeviceGroups(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	deviceGroupService := ctxServices.DeviceGroupsService
	tx := deviceGroupsFilters(r, db.DB)

	account, err := common.GetAccount(r)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error retrieving account from the request")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return
	}

	pagination := common.GetPagination(r)

	deviceGroupsCount, err := deviceGroupService.GetDeviceGroupsCount(account, tx)
	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	deviceGroups, err := deviceGroupService.GetDeviceGroups(account, pagination.Limit, pagination.Offset, tx)
	if err != nil {
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": deviceGroups, "count": deviceGroupsCount})
}

// CreateDeviceGroup is the route to create a new device group
func CreateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	deviceGroup, err := createDeviceRequest(w, r)
	if err != nil {
		return
	}
	ctxServices.Log.Info("Creating a device group")

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

// GetDeviceGroupByID return devices groups for an account and Id
func GetDeviceGroupByID(w http.ResponseWriter, r *http.Request) {
	if deviceGroup := getContextDeviceGroup(w, r); deviceGroup != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithJSONBody(w, ctxServices.Log, deviceGroup)
	}
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

// UpdateDeviceGroup updates the existing device group
func UpdateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	if oldDeviceGroup := getContextDeviceGroup(w, r); oldDeviceGroup != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		deviceGroup, err := createDeviceRequest(w, r)
		if err != nil {
			// error handled by createRequest already
			return
		}
		err = ctxServices.DeviceGroupsService.UpdateDeviceGroup(deviceGroup, oldDeviceGroup.Account, fmt.Sprint(oldDeviceGroup.ID))
		if err != nil {
			ctxServices.Log.WithField("error", err.Error()).Error("Error updating device group")
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

// DeleteDeviceGroupByID deletes an existing device group
func DeleteDeviceGroupByID(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	deviceGroup := getContextDeviceGroup(w, r)
	if deviceGroup == nil {
		return // error handled by getContextDeviceGroup already
	}
	ctxServices.Log = ctxServices.Log.WithField("device_group_id", deviceGroup.ID)
	ctxServices.Log.Info("Deleting a device group")
	err := ctxServices.DeviceGroupsService.DeleteDeviceGroupByID(fmt.Sprint(deviceGroup.ID))
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error deleting device group")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return
	}
	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"message": "Device group deleted"})
}

// createDeviceRequest validates request to create Device Group.
func createDeviceRequest(w http.ResponseWriter, r *http.Request) (*models.DeviceGroup, error) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	var deviceGroup *models.DeviceGroup
	if err := readRequestJSONBody(w, r, ctxServices.Log, &deviceGroup); err != nil {
		return nil, err
	}

	account, err := common.GetAccount(r)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Account was not set")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return nil, err
	}
	ctxServices.Log = ctxServices.Log.WithFields(log.Fields{
		"name":    deviceGroup.Name,
		"account": deviceGroup.Account,
	})
	deviceGroup.Account = account

	if err := deviceGroup.ValidateRequest(); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Info("Error validation request from device group")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return nil, err
	}
	return deviceGroup, nil
}

// AddDeviceGroupDevices add devices to device group
func AddDeviceGroupDevices(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	contextDeviceGroup := getContextDeviceGroup(w, r)
	if contextDeviceGroup == nil {
		return
	}

	var requestDeviceGroup models.DeviceGroup
	if err := readRequestJSONBody(w, r, ctxServices.Log, &requestDeviceGroup); err != nil {
		return
	}

	devicesAdded, err := ctxServices.DeviceGroupsService.AddDeviceGroupDevices(contextDeviceGroup.Account, contextDeviceGroup.ID, requestDeviceGroup.Devices)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error when adding deviceGroup devices")
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceGroupDevicesNotSupplied, *services.DeviceGroupAccountOrIDUndefined:
			apiError = errors.NewBadRequest(err.Error())
		case *services.DeviceGroupAccountDevicesNotFound:
			apiError = errors.NewNotFound(err.Error())
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	respondWithJSONBody(w, ctxServices.Log, devicesAdded)
}
