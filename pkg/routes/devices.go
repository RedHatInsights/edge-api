package routes

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.Route("/device/", func(r chi.Router) {
		r.Use(DeviceCtx)
		sub.Get("/{DeviceUUID}", GetDeviceStatus)
		sub.Get("/{DeviceUUID}/updates", GetUpdateAvailableForDevice)
	})
}

type deviceContextKey int

const DeviceContextKey deviceContextKey = iota

// DeviceContext implements context interfaces so we can shuttle around multiple values
type DeviceContext struct {
	DeviceUUID string
	// TODO: Implement devices by tag
	// Tag string
}

//  DeviceCtx is a handler for Device requests
func DeviceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uCtx DeviceContext
		uCtx.DeviceUUID = chi.URLParam(r, "DeviceUUID")
		if uCtx.DeviceUUID == "" {
			err := errors.NewBadRequest("DeviceUUID must be sent")
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		// TODO: Implement devices by tag
		// uCtx.Tag = chi.URLParam(r, "Tag")
		log.Debugf("UpdateCtx::uCtx: %#v", uCtx)
		ctx := context.WithValue(r.Context(), DeviceContextKey, &uCtx)
		log.Debugf("UpdateCtx::ctx: %#v", ctx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetDeviceStatus returns the device with the given UUID that is associate to the account.
// This is being used for the inventory table to determine whether the current device image
// is the latest or older version.
func GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	var results []models.Device
	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uuid := chi.URLParam(r, "DeviceUUID")
	result := db.DB.
		Select("desired_hash, connected, uuid").
		Table("devices").
		Joins(
			`JOIN updatetransaction_devices ON
			(updatetransaction_devices.device_id = devices.id AND devices.uuid = ?)`,
			uuid,
		).
		Joins(
			`JOIN update_transactions ON
			(
				update_transactions.id = updatetransaction_devices.update_transaction_id AND
				update_transactions.account = ?
			)`,
			account,
		).Find(&results)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(&results)
}

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func GetUpdateAvailableForDevice(w http.ResponseWriter, r *http.Request) {
	dc := r.Context().Value(DeviceContextKey).(DeviceContext)
	if dc.DeviceUUID == "" {
		return // Error set by DeviceCtx method
	}
	contextServices, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	result, err := contextServices.DeviceService.GetUpdateAvailableForDeviceByUUID(dc.DeviceUUID)
	if err == nil {
		json.NewEncoder(w).Encode(result)
		return
	}
	if _, ok := err.(*services.DeviceNotFoundError); ok {
		err := errors.NewNotFound("Could not find device")
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	log.WithFields(log.Fields{
		"requestId": request_id.GetReqID(r.Context()),
	}).Error(err)
	apierr := errors.NewInternalServerError()
	w.WriteHeader(apierr.Status)
	log.WithFields(log.Fields{
		"requestId":  request_id.GetReqID(r.Context()),
		"statusCode": apierr.Status,
	}).Error(apierr)
	json.NewEncoder(w).Encode(&err)
}
