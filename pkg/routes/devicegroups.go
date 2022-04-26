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
	sub.With(validateGetAllDeviceGroupsFilterParams).With(common.Paginate).Get("/", GetAllDeviceGroups)
	sub.Post("/", CreateDeviceGroup)
	sub.Get("/checkName/{name}", CheckGroupName)
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
			d.Get("/", GetDeviceGroupDetailsByIDView)
		})
		r.Route("/devices/{DEVICE_ID}", func(d chi.Router) {
			d.Use(DeviceGroupDeviceCtx)
			d.Delete("/", DeleteDeviceGroupOneDevice)
		})
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
			account, err := common.GetAccount(r)
			if err != nil || deviceGroup.DeviceGroup.Account != account {
				ctxServices.Log.WithFields(log.Fields{
					"error":   err.Error(),
					"account": account,
				}).Error("Error retrieving account or device group doesn't belong to account")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
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
			account, err := common.GetAccount(r)
			if err != nil {
				ctxServices.Log.WithFields(log.Fields{"error": err.Error()}).Error("Error retrieving account or device group doesn't belong to account")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
				return
			}
			deviceGroupDevice, err := ctxServices.DeviceGroupsService.GetDeviceGroupDeviceByID(account, deviceGroup.ID, uint(deviceID))
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

func respondWithAPIError(w http.ResponseWriter, logEntry *log.Entry, apiError errors.APIError) {
	w.WriteHeader(apiError.GetStatus())
	if err := json.NewEncoder(w).Encode(&apiError); err != nil {
		logEntry.WithField("error", err.Error()).Error("Error while trying to encode api error")
	}
}

func respondWithJSONBody(w http.ResponseWriter, logEntry *log.Entry, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logEntry.WithField("error", data).Error("Error while trying to encode")
		respondWithAPIError(w, logEntry, errors.NewInternalServerError())
	}
}

func readRequestJSONBody(w http.ResponseWriter, r *http.Request, logEntry *log.Entry, dataReceiver interface{}) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dataReceiver); err != nil {
		logEntry.WithField("error", err.Error()).Error("Error parsing json from device group request")
		respondWithAPIError(w, logEntry, errors.NewBadRequest("invalid JSON request"))
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

// GetDeviceGroupDetailsByID return devices groups details for an account and Id
func GetDeviceGroupDetailsByID(w http.ResponseWriter, r *http.Request) {
	if deviceGroup := getContextDeviceGroupDetails(w, r); deviceGroup != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithJSONBody(w, ctxServices.Log, deviceGroup)
	}
}

// GetDeviceGroupDetailsByIDView return devices groups details for an account and Id
func GetDeviceGroupDetailsByIDView(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	deviceGroup := getContextDeviceGroup(w, r)
	if deviceGroup == nil {
		return
	}

	devicesIDS := make([]uint, 0, len(deviceGroup.Devices))
	for _, device := range deviceGroup.Devices {
		devicesIDS = append(devicesIDS, device.ID)
	}
	tx := devicesFilters(r, db.DB).
		Where("image_id IS NOT NULL AND image_id != 0 AND ID IN (?)", devicesIDS)
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
	var deviceGroupDetails models.DeviceGroupDetailsView
	deviceGroupDetails.DeviceGroup = deviceGroup
	deviceGroupDetails.DeviceDetails.Devices = deviceGroupDevices.Devices
	deviceGroupDetails.DeviceDetails.Total = len(deviceGroupDevices.Devices)

	respondWithJSONBody(w, ctxServices.Log, &deviceGroupDetails)
}

// GetDeviceGroupByID return devices groups for an account and Id
func GetDeviceGroupByID(w http.ResponseWriter, r *http.Request) {
	if deviceGroup := getContextDeviceGroup(w, r); deviceGroup != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
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
	ctxLog := ctxServices.Log.WithField("device_group_id", deviceGroup.ID)
	ctxLog.Info("Deleting a device group")
	err := ctxServices.DeviceGroupsService.DeleteDeviceGroupByID(fmt.Sprint(deviceGroup.ID))
	if err != nil {
		ctxLog.WithField("error", err.Error()).Error("Error deleting device group")
		var apiError errors.APIError
		switch err.(type) {
		case *services.AccountNotSet:
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

// DeleteDeviceGroupManyDevices delete the requested devices from device-group
func DeleteDeviceGroupManyDevices(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	contextDeviceGroup := getContextDeviceGroup(w, r)
	if contextDeviceGroup == nil {
		return
	}

	var requestDeviceGroup models.DeviceGroup
	if err := readRequestJSONBody(w, r, ctxServices.Log, &requestDeviceGroup); err != nil {
		return
	}

	deletedDevices, err := ctxServices.DeviceGroupsService.DeleteDeviceGroupDevices(contextDeviceGroup.Account, contextDeviceGroup.ID, requestDeviceGroup.Devices)
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error when removing deviceGroup devices")
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceGroupDevicesNotSupplied, *services.DeviceGroupAccountOrIDUndefined:
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

// DeleteDeviceGroupOneDevice delete the requested device from device-group
func DeleteDeviceGroupOneDevice(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	contextDeviceGroup := getContextDeviceGroup(w, r)
	contextDeviceGroupDevice := getContextDeviceGroupDevice(w, r)
	if contextDeviceGroupDevice == nil {
		return
	}

	_, err := ctxServices.DeviceGroupsService.DeleteDeviceGroupDevices(
		contextDeviceGroup.Account, contextDeviceGroup.ID, []models.Device{*contextDeviceGroupDevice},
	)

	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error when removing deviceGroup devices")
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceGroupDevicesNotSupplied, *services.DeviceGroupAccountOrIDUndefined:
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

// CheckGroupName validates if a group name exists on an account
func CheckGroupName(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	var name = chi.URLParam(r, "name")

	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithFields(log.Fields{
			"error":   err.Error(),
			"account": account,
		}).Error("Error retrieving account")
		respondWithAPIError(w, services.Log, errors.NewBadRequest(err.Error()))
		return
	}

	value, err := services.DeviceGroupsService.DeviceGroupNameExists(account, name)

	if err != nil {
		respondWithAPIError(w, services.Log, errors.NewBadRequest(err.Error()))
	}

	respondWithJSONBody(w, services.Log, map[string]interface{}{"data": map[string]interface{}{"isValid": value}})
}
