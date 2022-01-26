package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.Get("/", GetDevices)
	sub.Route("/{DeviceUUID}", func(r chi.Router) {
		r.Use(DeviceCtx)
		r.Get("/", GetDevice)
		r.Get("/updates", GetUpdateAvailableForDevice)
		r.Get("/image", GetDeviceImageInfo)
	})
}

type deviceContextKey int

// DeviceContextKey is the key to DeviceContext (for Device requests)
const DeviceContextKey deviceContextKey = iota

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
		ctx := context.WithValue(r.Context(), DeviceContextKey, dc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func GetUpdateAvailableForDevice(w http.ResponseWriter, r *http.Request) {
	s := dependencies.ServicesFromContext(r.Context())
	dc, ok := r.Context().Value(DeviceContextKey).(DeviceContext)
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
	dc, ok := r.Context().Value(DeviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	result, err := s.DeviceService.GetDeviceImageInfo(dc.DeviceUUID)
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
	dc, ok := r.Context().Value(DeviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" || !ok {
		return // Error set by DeviceCtx method
	}
	result, err := s.DeviceService.GetDeviceDetails(dc.DeviceUUID)
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
	param.DeviceStatus = v.Get("device_status")
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
