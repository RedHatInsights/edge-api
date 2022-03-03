package routes

import (
	"context"
	"encoding/json"
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
	log "github.com/sirupsen/logrus"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.Get("/", GetDevices)
	sub.Get("/db", GetDBDevices) //tmp validation
	sub.Route("/{DeviceUUID}", func(r chi.Router) {
		r.Use(DeviceCtx)
		r.Get("/", GetDevice)
		r.Get("/dbinfo", GetDeviceDBInfo)
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
			err := errors.NewBadRequest("DeviceUUID must be sent")
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
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
	s := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	result, err := s.DeviceService.GetUpdateAvailableForDeviceByUUID(dc.DeviceUUID)
	if err == nil {
		if err := json.NewEncoder(w).Encode(result); err != nil {
			s.Log.WithField("error", result).Error("Error while trying to encode")
		}
		return
	}
	if _, ok := err.(*services.DeviceNotFoundError); ok {
		err := errors.NewNotFound("Could not find device")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	if _, ok := err.(*services.UpdateNotFoundError); ok {
		err := errors.NewNotFound("Could not find update")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.GetStatus())
	s.Log.WithFields(log.Fields{
		"statusCode": apierr.GetStatus(),
		"error":      apierr.Error(),
	}).Error("Error retrieving updates for device")
	if err := json.NewEncoder(w).Encode(&err); err != nil {
		s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
	}
}

// GetDeviceImageInfo returns the information of a running image for a device
func GetDeviceImageInfo(w http.ResponseWriter, r *http.Request) {
	s := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	result, err := s.DeviceService.GetDeviceImageInfoByUUID(dc.DeviceUUID)
	if err == nil {
		if err := json.NewEncoder(w).Encode(result); err != nil {
			s.Log.WithField("error", result).Error("Error while trying to encode")
		}
		return
	}
	if _, ok := err.(*services.DeviceNotFoundError); ok {
		err := errors.NewNotFound("Could not find device")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.GetStatus())
	s.Log.WithFields(log.Fields{
		"statusCode": apierr.GetStatus(),
		"error":      apierr.Error(),
	}).Error("Error getting image info for device")
	if err := json.NewEncoder(w).Encode(&err); err != nil {
		s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
	}
}

// GetDevice returns all available information that edge api has about a device
// It returns the information stored on our database and the device ID on our side, if any.
// Returns the information of a running image and previous image in case of a rollback.
// Returns updates available to a device.
// Returns updates transactions for that device, if any.
func GetDevice(w http.ResponseWriter, r *http.Request) {
	s := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	result, err := s.DeviceService.GetDeviceDetailsByUUID(dc.DeviceUUID)
	if err == nil {
		if err := json.NewEncoder(w).Encode(result); err != nil {
			s.Log.WithField("error", result).Error("Error while trying to encode")
		}
		return
	}
	if _, ok := err.(*services.ImageNotFoundError); ok {
		err := errors.NewNotFound("Could not find image")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	if _, ok := err.(*services.DeviceNotFoundError); ok {
		err := errors.NewNotFound("Could not find device")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.GetStatus())
	s.Log.WithFields(log.Fields{
		"statusCode": apierr.GetStatus(),
		"error":      apierr.Error(),
	}).Error("Error retrieving updates for device")
	if err := json.NewEncoder(w).Encode(&err); err != nil {
		s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
	}
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
	services := dependencies.ServicesFromContext(r.Context())
	params := deviceListFilters(r.URL.Query())
	inventory, err := services.DeviceService.GetDevices(params)
	if err != nil || inventory.Count == 0 {
		err := errors.NewNotFound("No devices found")
		w.WriteHeader(err.GetStatus())
		_ = json.NewEncoder(w).Encode(err)
		return
	}
	if err := json.NewEncoder(w).Encode(inventory); err != nil {
		services := dependencies.ServicesFromContext(r.Context())
		services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		_ = json.NewEncoder(w).Encode(err)
	}
}

// GetDBDevices return the device data on EdgeAPI DB
func GetDBDevices(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	var devices *[]models.Device
	pagination := common.GetPagination(r)
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err).Debug("Account not found")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	result := db.DB.Limit(pagination.Limit).Offset(pagination.Offset).Where("account = ?", account).Find(&devices)
	if result.Error != nil {
		services.Log.WithField("error", result.Error.Error()).Debug("Result error")
		err := errors.NewBadRequest(result.Error.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", result.Error.Error()).Error("Error while trying to encode")
		}
		return
	}
	if err := json.NewEncoder(w).Encode(devices); err != nil {
		services := dependencies.ServicesFromContext(r.Context())
		services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		_ = json.NewEncoder(w).Encode(err)
	}

}

// GetDeviceDBInfo return the device data on EdgeAPI DB
func GetDeviceDBInfo(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	var devices *[]models.Device
	// pagination := common.GetPagination(r)
	dc, ok := r.Context().Value(deviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err).Debug("Account not found")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	result := db.DB.Where("account = ? and UUID = ?", account, dc.DeviceUUID).Find(&devices)
	if result.Error != nil {
		services.Log.WithField("error", err).Debug("Result error")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	if err := json.NewEncoder(w).Encode(devices); err != nil {
		services := dependencies.ServicesFromContext(r.Context())
		services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		_ = json.NewEncoder(w).Encode(err)
	}

}
