package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.Get("/", GetInventory) // TODO: Still a proof-of-concept and needs to be refactored in the future
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
			json.NewEncoder(w).Encode(&err)
			return
		}
		// TODO: Implement devices by tag
		// dc.Tag = chi.URLParam(r, "Tag")
		ctx := context.WithValue(r.Context(), DeviceContextKey, dc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getDevice(w http.ResponseWriter, r *http.Request) *models.Device {
	ctx := r.Context()
	dc, ok := ctx.Value(DeviceContextKey).(*DeviceContext)
	if dc.DeviceUUID == "" {
		return nil // Error set by DeviceCtx method
	}
	if !ok {
		err := errors.NewBadRequest("Must pass device identifier")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil
	}
	s := dependencies.ServicesFromContext(r.Context())
	s.Log = s.Log.WithField("UUID", dc.DeviceUUID)
	device, err := s.DeviceService.GetDeviceByUUID(dc.DeviceUUID)
	if _, ok := err.(*services.DeviceNotFoundError); ok {
		err := errors.NewNotFound("Could not find device")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil
	}
	return device
}

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func GetUpdateAvailableForDevice(w http.ResponseWriter, r *http.Request) {
	if device := getDevice(w, r); device != nil {
		s, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
		result, err := s.DeviceService.GetUpdateAvailableForDeviceByUUID(device.UUID)
		if err == nil {
			json.NewEncoder(w).Encode(result)
			return
		}
		if _, ok := err.(*services.DeviceNotFoundError); ok {
			err := errors.NewNotFound("Could not find device")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		if _, ok := err.(*services.UpdateNotFoundError); ok {
			err := errors.NewNotFound("Could not find update")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		apierr := errors.NewInternalServerError()
		w.WriteHeader(apierr.GetStatus())
		s.Log.WithFields(log.Fields{
			"statusCode": apierr.GetStatus(),
			"error":      apierr.Error(),
		}).Error("Error retrieving updates for device")
		json.NewEncoder(w).Encode(&err)
	}
}

// GetDeviceImageInfo returns the information of a running image for a device
func GetDeviceImageInfo(w http.ResponseWriter, r *http.Request) {
	if device := getDevice(w, r); device != nil {
		s := dependencies.ServicesFromContext(r.Context())
		result, err := s.DeviceService.GetDeviceImageInfo(device.UUID)
		if err == nil {
			json.NewEncoder(w).Encode(result)
			return
		}
		if _, ok := err.(*services.DeviceNotFoundError); ok {
			err := errors.NewNotFound("Could not find device")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		apierr := errors.NewInternalServerError()
		w.WriteHeader(apierr.GetStatus())
		s.Log.WithFields(log.Fields{
			"statusCode": apierr.GetStatus(),
			"error":      apierr.Error(),
		}).Error("Error getting image info for device")
		json.NewEncoder(w).Encode(&err)
	}
}

// GetDevice returns all available information that edge api has about a device
// It returns the information stored on our database and the device ID on our side, if any.
// Returns the information of a running image and previous image in case of a rollback.
// Returns updates available to a device.
// Returns updates transactions for that device, if any.
func GetDevice(w http.ResponseWriter, r *http.Request) {
	if device := getDevice(w, r); device != nil {
		s := dependencies.ServicesFromContext(r.Context())
		result, err := s.DeviceService.GetDeviceDetails(device.UUID)
		if err == nil {
			json.NewEncoder(w).Encode(result)
			return
		}
		if _, ok := err.(*services.ImageNotFoundError); ok {
			err := errors.NewNotFound("Could not find image")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		if _, ok := err.(*services.DeviceNotFoundError); ok {
			err := errors.NewNotFound("Could not find device")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		apierr := errors.NewInternalServerError()
		w.WriteHeader(apierr.GetStatus())
		s.Log.WithFields(log.Fields{
			"statusCode": apierr.GetStatus(),
			"error":      apierr.Error(),
		}).Error("Error retrieving updates for device")
		json.NewEncoder(w).Encode(&err)
	}
}
