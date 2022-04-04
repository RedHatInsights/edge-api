package routes

import (
	"context"
	"net/http"
	"net/url"

	"github.com/go-chi/chi"
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
	sub.Get("/", GetDevices)
	sub.Get("/devicesview", GetDevicesView)
	sub.With(common.Paginate).Get("/db", GetDBDevices)
	sub.Route("/{DeviceUUID}", func(r chi.Router) {
		r.Use(DeviceCtx)
		r.Get("/dbinfo", GetDeviceDBInfo)
		r.Get("/", GetDevice)
		r.Get("/updates", GetUpdateAvailableForDevice)
		r.Get("/image", GetDeviceImageInfo)
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

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func GetUpdateAvailableForDevice(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	result, err := contextServices.DeviceService.GetUpdateAvailableForDeviceByUUID(dc.DeviceUUID)
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
	result, err := contextServices.DeviceService.GetDeviceImageInfoByUUID(dc.DeviceUUID)
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
	result, err := contextServices.DeviceService.GetDeviceDetailsByUUID(dc.DeviceUUID)
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
	account, err := common.GetAccount(r)
	if err != nil {
		contextServices.Log.WithField("error", err).Debug("Account not found")
		respondWithAPIError(w, contextServices.Log, errors.NewBadRequest(err.Error()))
		return
	}
	result := db.DB.Limit(pagination.Limit).Offset(pagination.Offset).Where("account = ?", account).Find(&devices)
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
	account, err := common.GetAccount(r)
	if err != nil {
		contextServices.Log.WithField("error", err).Debug("Account not found")
		respondWithAPIError(w, contextServices.Log, errors.NewBadRequest(err.Error()))
		return
	}
	result := db.DB.Where("account = ? and UUID = ?", account, dc.DeviceUUID).Find(&devices)
	if result.Error != nil {
		contextServices.Log.WithField("error", result.Error).Debug("Result error")
		respondWithAPIError(w, contextServices.Log, errors.NewBadRequest(result.Error.Error()))
		return
	}
	respondWithJSONBody(w, contextServices.Log, &devices)
}

// GetDevicesView returns all data needed to display customers devices
func GetDevicesView(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	params := deviceListFilters(r.URL.Query())
	devicesViewList, err := contextServices.DeviceService.GetDevicesView(params)
	if err != nil {
		respondWithAPIError(w, contextServices.Log, errors.NewNotFound("No devices found"))
		return
	}
	respondWithJSONBody(w, contextServices.Log, devicesViewList)
}
