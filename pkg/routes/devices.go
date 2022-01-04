package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.Get("/", GetInventory)
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
		var uCtx DeviceContext
		uCtx.DeviceUUID = chi.URLParam(r, "DeviceUUID")
		if uCtx.DeviceUUID == "" {
			err := errors.NewBadRequest("DeviceUUID must be sent")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		// TODO: Implement devices by tag
		// uCtx.Tag = chi.URLParam(r, "Tag")
		log.Debugf("UpdateCtx::uCtx: %#v", uCtx)
		ctx := context.WithValue(r.Context(), DeviceContextKey, uCtx)
		log.Debugf("UpdateCtx::ctx: %#v", ctx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func GetUpdateAvailableForDevice(w http.ResponseWriter, r *http.Request) {
	dc := r.Context().Value(DeviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" {
		return // Error set by DeviceCtx method
	}
	contextServices := dependencies.ServicesFromContext(r.Context())
	result, err := contextServices.DeviceService.GetUpdateAvailableForDeviceByUUID(dc.DeviceUUID)
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
	log.WithFields(log.Fields{
		"requestId": request_id.GetReqID(r.Context()),
	}).Error(err)
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.GetStatus())
	log.WithFields(log.Fields{
		"requestId":  request_id.GetReqID(r.Context()),
		"statusCode": apierr.GetStatus(),
	}).Error(apierr)
	json.NewEncoder(w).Encode(&err)
}

// GetDeviceImageInfo returns the information of a running image for a device
func GetDeviceImageInfo(w http.ResponseWriter, r *http.Request) {
	dc := r.Context().Value(DeviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" {
		return // Error set by DeviceCtx method
	}
	contextServices := dependencies.ServicesFromContext(r.Context())
	result, err := contextServices.DeviceService.GetDeviceImageInfo(dc.DeviceUUID)
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
	log.WithFields(log.Fields{
		"requestId": request_id.GetReqID(r.Context()),
	}).Error(err)
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.GetStatus())
	log.WithFields(log.Fields{
		"requestId":  request_id.GetReqID(r.Context()),
		"statusCode": apierr.GetStatus(),
	}).Error(apierr)
	json.NewEncoder(w).Encode(&err)
}

// GetDevice returns all available information that edge api has about a device
// It returns the information stored on our database and the device ID on our side, if any.
// Returns the information of a running image and previous image in case of a rollback.
// Returns updates available to a device.
// Returns updates transactions for that device, if any.
func GetDevice(w http.ResponseWriter, r *http.Request) {
	dc := r.Context().Value(DeviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" {
		return // Error set by DeviceCtx method
	}
	contextServices := dependencies.ServicesFromContext(r.Context())
	result, err := contextServices.DeviceService.GetDeviceDetails(dc.DeviceUUID)
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
	log.WithFields(log.Fields{
		"requestId": request_id.GetReqID(r.Context()),
	}).Error(err)
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.GetStatus())
	log.WithFields(log.Fields{
		"requestId":  request_id.GetReqID(r.Context()),
		"statusCode": apierr.GetStatus(),
	}).Error(apierr)
	json.NewEncoder(w).Encode(&err)
}
