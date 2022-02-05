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

type deviceGroupTypeKey int

const deviceGroupKey deviceGroupTypeKey = iota

// MakeDeviceGroupsRouter adds support for device groups operations
func MakeDeviceGroupsRouter(sub chi.Router) {
	sub.With(validateGetAllDeviceGroupsFilterParams).With(common.Paginate).Get("/", GetAllDeviceGroups)
	sub.Post("/", CreateDeviceGroup)
	sub.Route("/{ID}", func(r chi.Router) {
		r.Use(DeviceGroupCtx)
		r.Get("/", GetDeviceGroupByID)
		r.Put("/", UpdateDeviceGroup)
		r.Delete("/", DeleteDeviceGroupByID)
	})
}

// DeviceGroupCtx is a handler to Device Group requests
func DeviceGroupCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := dependencies.ServicesFromContext(r.Context())
		if ID := chi.URLParam(r, "ID"); ID != "" {
			_, err := strconv.Atoi(ID)
			s.Log = s.Log.WithField("deviceGroupID", ID)
			s.Log.Debug("Retrieving device group")
			if err != nil {
				s.Log.Debug("ID is not an integer")
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				if err := json.NewEncoder(w).Encode(&err); err != nil {
					services := dependencies.ServicesFromContext(r.Context())
					services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
				}
				return
			}

			deviceGroup, err := s.DeviceGroupsService.GetDeviceGroupByID(ID)
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.DeviceGroupNotFound:
					responseErr = errors.NewNotFound(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
					responseErr.SetTitle("failed getting third device group")
				}
				w.WriteHeader(responseErr.GetStatus())
				if err := json.NewEncoder(w).Encode(&responseErr); err != nil {
					s.Log.WithField("error", responseErr.Error()).Error("Error while trying to encode")
				}
				return
			}
			account, err := common.GetAccount(r)
			if err != nil || deviceGroup.Account != account {
				s.Log.WithFields(log.Fields{
					"error":   err.Error(),
					"account": account,
				}).Error("Error retrieving account or device group doesn't belong to account")
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				if err := json.NewEncoder(w).Encode(&err); err != nil {
					s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
				}
				return
			}
			ctx := context.WithValue(r.Context(), deviceGroupKey, deviceGroup)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			s.Log.Debug("deviceGroup ID was not passed to the request or it was empty")
			err := errors.NewBadRequest("deviceGroup ID required")
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				s.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
			return
		}
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
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		return
	}
	services.Log.Info("Creating a device group")

	deviceGroup, err = services.DeviceGroupsService.CreateDeviceGroup(deviceGroup)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error creating a device group")
		err := errors.NewInternalServerError()
		err.SetTitle("failed creating device group")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(&deviceGroup); err != nil {
		services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
	}
}

// GetDeviceGroupByID return devices groups for an account and Id
func GetDeviceGroupByID(w http.ResponseWriter, r *http.Request) {
	if deviceGroup := getDeviceGroups(w, r); deviceGroup != nil {
		if err := json.NewEncoder(w).Encode(deviceGroup); err != nil {
			services := dependencies.ServicesFromContext(r.Context())
			services.Log.WithField("error", deviceGroup).Error("Error while trying to encode")
		}
	}

}
func getDeviceGroups(w http.ResponseWriter, r *http.Request) *models.DeviceGroup {
	ctx := r.Context()
	deviceGroup, ok := ctx.Value(deviceGroupKey).(*models.DeviceGroup)

	if !ok {
		err := errors.NewBadRequest("Failed getting device group from context")
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return nil
	}
	return deviceGroup
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

	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Account was not set")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
	}
	services.Log = services.Log.WithFields(log.Fields{
		"name":    deviceGroup.Name,
		"account": deviceGroup.Account,
	})
	deviceGroup.Account = account

	if err := deviceGroup.ValidateRequest(); err != nil {
		services.Log.WithField("error", err.Error()).Info("Error validation request from device group")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		return nil, err
	}
	return deviceGroup, nil
}
