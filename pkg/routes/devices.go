// FIXME: golangci-lint
// nolint:govet,revive
package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("devices")).With(ValidateGetAllDevicesFilterParams).Get("/", GetDevices)
	sub.With(ValidateQueryParams("devicesview")).With(common.Paginate).With(ValidateGetDevicesViewFilterParams).Get("/devicesview", GetDevicesView)
	sub.With(common.Paginate).Get("/db", GetDBDevices)
	sub.Route("/{DeviceUUID}", func(r chi.Router) {
		r.Use(DeviceCtx)
		r.Get("/dbinfo", GetDeviceDBInfo)
		r.With(common.Paginate).Get("/", GetDevice)
		r.With(common.Paginate).Get("/updates", GetUpdateAvailableForDevice)
		r.With(common.Paginate).Get("/image", GetDeviceImageInfo)
	})
}

type deviceContextKeyType string

// deviceContextKey is the key to DeviceContext (for Device requests)
const deviceContextKey = deviceContextKeyType("device_context_key")

// DeviceContext implements context interfaces so we can shuttle around multiple values
type DeviceContext struct {
	DeviceUUID string
	// TODO: Implement devices by tag
	// Tag string
}

// DeviceCtx is a handler for Device requests
func DeviceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var dc DeviceContext
		dc.DeviceUUID = chi.URLParam(r, "DeviceUUID")
		if dc.DeviceUUID == "" {
			contextServices := dependencies.ServicesFromContext(r.Context())
			respondWithAPIError(w, contextServices.Log, errors.NewBadRequest("DeviceUUID must be sent"))
			return
		}
		// TODO: Implement devices by tag
		// dc.Tag = chi.URLParam(r, "Tag")
		ctx := context.WithValue(r.Context(), deviceContextKey, dc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var devicesFilters = common.ComposeFilters(
	// Filter handler for "name"
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "devices.name",
	}),
	// Filter handler for "uuid"
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "uuid",
		DBField:    "devices.uuid",
	}),
	// Filter handler for "update_available"
	common.BoolFilterHandler(&common.Filter{
		QueryParam: "update_available",
		DBField:    "devices.update_available",
	}),
	// Filter handler for "created_at"
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "devices.created_at",
	}),
	// Filter handler for "image_id"
	common.IntegerNumberFilterHandler(&common.Filter{
		QueryParam: "image_id",
		DBField:    "devices.image_id",
	}),
	common.SortFilterHandler("devices", "name", "ASC"),
)

// ValidateGetAllDevicesFilterParams validate the query params that sent to /devices endpoint
func ValidateGetAllDevicesFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var errs []validationError
		// "uuid" validation
		if val := r.URL.Query().Get("uuid"); val != "" {
			if _, err := uuid.Parse(val); err != nil {
				errs = append(errs, validationError{Key: "uuid", Reason: err.Error()})
			}
		}
		// "created_at" validation
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()})
			}
		}
		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(&errs); err != nil {
			ctxServices := dependencies.ServicesFromContext(r.Context())
			ctxServices.Log.WithField("error", errs).Error("Error while trying to encode devices filter validation errors")
		}
	})
}

// ValidateGetDevicesViewFilterParams validate the query parameters that sent to /devicesview endpoint
func ValidateGetDevicesViewFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var device models.Device
		var errs []validationError
		ctxServices := dependencies.ServicesFromContext(r.Context())

		// check for invalid update_available value
		if val := r.URL.Query().Get("update_available"); val != "true" && val != "false" && val != "" {
			if !device.UpdateAvailable {
				errs = append(errs, validationError{Key: "update_available", Reason: fmt.Sprintf("%s is not a valid value for update_available. update_available must be boolean", val)})
			}
		}
		// check for invalid image_id value
		if val := r.URL.Query().Get("image_id"); val != "" {
			if _, err := strconv.Atoi(val); err != nil {
				errs = append(errs, validationError{Key: "image_id", Reason: fmt.Sprintf("%s is not a valid value for image_id. image_id must be integer", val)})
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

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func GetUpdateAvailableForDevice(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	// if 'latest' set in query, return the latest update available, aka latest = true
	latest := false
	if r.URL.Query().Get("latest") == "true" {
		latest = true
	}
	pagination := common.GetPagination(r)
	result, err := contextServices.DeviceService.GetUpdateAvailableForDeviceByUUID(dc.DeviceUUID, latest, pagination.Limit, pagination.Offset)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceNotFoundError:
			apiError = errors.NewNotFound("Could not find device")
		case *services.UpdateNotFoundError:
			apiError = errors.NewNotFound("Could not find update")
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, contextServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, contextServices.Log, result)
}

// GetDeviceImageInfo returns the information of a running image for a device
func GetDeviceImageInfo(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	pagination := common.GetPagination(r)
	result, err := contextServices.DeviceService.GetDeviceImageInfoByUUID(dc.DeviceUUID, pagination.Limit, pagination.Offset)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.DeviceNotFoundError:
			apiError = errors.NewNotFound("Could not find device")
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, contextServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, contextServices.Log, result)
}

// GetDevice returns all available information that edge api has about a device
// It returns the information stored on our database and the device ID on our side, if any.
// Returns the information of a running image and previous image in case of a rollback.
// Returns updates available to a device.
// Returns updates transactions for that device, if any.
func GetDevice(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	pagination := common.GetPagination(r)
	result, err := contextServices.DeviceService.GetDeviceDetailsByUUID(dc.DeviceUUID, pagination.Limit, pagination.Offset)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ImageNotFoundError:
			apiError = errors.NewNotFound("Could not find image")
		case *services.DeviceNotFoundError:
			apiError = errors.NewNotFound("Could not find device")
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, contextServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, contextServices.Log, result)
}

// InventoryData represents the structure of inventory response
type InventoryData struct {
	Total   int
	Count   int
	Page    int
	PerPage int
	Results []InventoryResponse
}

// InventoryResponse represents the structure of inventory data on response
type InventoryResponse struct {
	ID         string
	DeviceName string
	LastSeen   string
	ImageInfo  *models.ImageInfo
}

func deviceListFilters(v url.Values) *inventory.Params {
	var param *inventory.Params = new(inventory.Params)
	param.PerPage = v.Get("per_page")
	param.Page = v.Get("page")
	param.OrderBy = v.Get("order_by")
	param.OrderHow = v.Get("order_how")
	param.HostnameOrID = v.Get("hostname_or_id")
	// TODO: Plan and figure out how to filter this properly
	// param.DeviceStatus = v.Get("device_status")
	return param
}

// GetDevices return the device data both on Edge API and InventoryAPI
func GetDevices(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	params := deviceListFilters(r.URL.Query())
	inventory, err := contextServices.DeviceService.GetDevices(params)
	if err != nil {
		respondWithAPIError(w, contextServices.Log, errors.NewNotFound("No devices found"))
		return
	}
	respondWithJSONBody(w, contextServices.Log, inventory)
}

// GetDBDevices return the device data on EdgeAPI DB
func GetDBDevices(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	var devices []models.Device
	pagination := common.GetPagination(r)
	orgID := readOrgID(w, r, contextServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}
	result := db.Org(orgID, "").Limit(pagination.Limit).Offset(pagination.Offset).Find(&devices)
	if result.Error != nil {
		contextServices.Log.WithField("error", result.Error.Error()).Debug("Result error")
		respondWithAPIError(w, contextServices.Log, errors.NewBadRequest(result.Error.Error()))
		return
	}
	respondWithJSONBody(w, contextServices.Log, &devices)
}

// GetDeviceDBInfo return the device data on EdgeAPI DB
func GetDeviceDBInfo(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	var devices []models.Device
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	orgID := readOrgID(w, r, contextServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}
	if result := db.Org(orgID, "").Where("UUID = ?", dc.DeviceUUID).Find(&devices); result.Error != nil {
		contextServices.Log.WithField("error", result.Error).Debug("Result error")
		respondWithAPIError(w, contextServices.Log, errors.NewBadRequest(result.Error.Error()))
		return
	}
	respondWithJSONBody(w, contextServices.Log, &devices)
}

// GetDevicesView returns all data needed to display customers devices
func GetDevicesView(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	tx := devicesFilters(r, db.DB.Debug()).Where("image_id IS NOT NULL AND image_id != 0")
	pagination := common.GetPagination(r)

	devicesCount, err := contextServices.DeviceService.GetDevicesCount(tx)
	if err != nil {
		respondWithAPIError(w, contextServices.Log, errors.NewNotFound("No devices found"))
		return
	}

	devicesViewList, err := contextServices.DeviceService.GetDevicesView(pagination.Limit, pagination.Offset, tx)
	if err != nil {
		respondWithAPIError(w, contextServices.Log, errors.NewNotFound("No devices found"))
		return
	}
	respondWithJSONBody(w, contextServices.Log, map[string]interface{}{"data": devicesViewList, "count": devicesCount})
}
