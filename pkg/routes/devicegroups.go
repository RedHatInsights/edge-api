package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// MakeDeviceGroupsRouter adds support for device groups operations
func MakeDeviceGroupsRouter(sub chi.Router) {
	sub.With(validateGetAllDeviceGroupsFilterParams).With(common.Paginate).Get("/", GetAllDeviceGroups)
	sub.Post("/", CreateDeviceGroup)
	sub.Route("/{ID}", func(r chi.Router) {
		// MUST ADD CONTEXT
		r.Get("/", GetDeviceGroupByID)
		r.Put("/", UpdateDeviceGroup)
		r.Delete("/", DeleteDeviceGroupByID)
	})
}

func logRequestAPIError(w http.ResponseWriter, log *log.Entry, apiError errors.APIError) {
	w.WriteHeader(apiError.GetStatus())
	if err := json.NewEncoder(w).Encode(&apiError); err != nil {
		log.WithField("error", err.Error()).Error("Error while trying to encode api error")
	}
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
			services := dependencies.ServicesFromContext(r.Context())
			services.Log.WithField("error", errs).Error("Error while trying to encode device groups filter validation errors")
		}
	})
}

// GetAllDeviceGroups return devices groups for an account
func GetAllDeviceGroups(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	deviceGroupService := services.DeviceGroupsService
	tx := deviceGroupsFilters(r, db.DB)

	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error retrieving account from the request")
		logRequestAPIError(w, services.Log, errors.NewBadRequest(err.Error()))
		return
	}

	pagination := common.GetPagination(r)

	deviceGroupsCount, err := deviceGroupService.GetDeviceGroupsCount(account, tx)
	if err != nil {
		logRequestAPIError(w, services.Log, errors.NewInternalServerError())
		return
	}

	deviceGroups, err := deviceGroupService.GetDeviceGroups(account, pagination.Limit, pagination.Offset, tx)
	if err != nil {
		logRequestAPIError(w, services.Log, errors.NewInternalServerError())
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{"data": deviceGroups, "count": deviceGroupsCount}); err != nil {
		services.Log.WithField("error", map[string]interface{}{"data": &deviceGroups, "count": deviceGroupsCount}).Error("Error while trying to encode")
	}
}

// CreateDeviceGroup is the route to create a new device group
func CreateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()
	deviceGroup, err := createDeviceRequest(w, r)
	if err != nil {
		// error handled by createRequest already
		return
	}
	services.Log.Info("Creating a device group")

	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Account was not set")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}

	deviceGroup, err = services.DeviceGroupsService.CreateDeviceGroup(deviceGroup, account)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error creating a device group")
		err := errors.NewInternalServerError()
		err.SetTitle("failed creating third party repository")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(&deviceGroup); err != nil {
		services.Log.WithField("error", deviceGroup).Error("Error while trying to encode")
	}
}

// GetDeviceGroupByID return devices groups for an account and Id
func GetDeviceGroupByID(w http.ResponseWriter, r *http.Request) {

}

// UpdateDeviceGroup updates the existing device group
func UpdateDeviceGroup(w http.ResponseWriter, r *http.Request) {

}

// DeleteDeviceGroupByID deletes an existing device group
func DeleteDeviceGroupByID(w http.ResponseWriter, r *http.Request) {

}

// createDeviceRequest validates request to create Device Group.
func createDeviceRequest(w http.ResponseWriter, r *http.Request) (*models.DeviceGroup, error) {
	services := dependencies.ServicesFromContext(r.Context())

	var deviceGroup *models.DeviceGroup
	if err := json.NewDecoder(r.Body).Decode(&deviceGroup); err != nil {
		services.Log.WithField("error", err.Error()).Error("Error parsing json from device group")
		err := errors.NewBadRequest("invalid JSON request")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return nil, err
	}
	services.Log = services.Log.WithFields(log.Fields{
		"name": deviceGroup.Name,
	})

	if err := deviceGroup.ValidateRequest(); err != nil {
		services.Log.WithField("error", err.Error()).Info("Error validation request from device group")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		return nil, err
	}
	return deviceGroup, nil
}
