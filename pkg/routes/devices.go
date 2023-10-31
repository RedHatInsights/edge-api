// FIXME: golangci-lint
// nolint:govet,revive
package routes

import (
	"context"
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
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	"gorm.io/gorm"
)

// MakeDevicesRouter adds support for operations on update
func MakeDevicesRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("devices")).With(ValidateGetAllDevicesFilterParams).Get("/", GetDevices)
	sub.With(ValidateQueryParams("devicesview")).With(common.Paginate).With(ValidateGetDevicesViewFilterParams).Get("/devicesview", GetDevicesView)
	sub.With(ValidateQueryParams("devicesview")).With(common.Paginate).With(ValidateGetDevicesViewFilterParams).Post("/devicesview", GetDevicesViewWithinDevices)
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
	// Filter handler for "groupUUID"
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "groupUUID",
		DBField:    "devices.group_uuid",
	}),
	common.SortFilterHandler("devices", "name", "ASC"),
)

// ValidateGetAllDevicesFilterParams validate the query params that sent to /devices endpoint
func ValidateGetAllDevicesFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
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
		respondWithJSONBody(w, ctxServices.Log, &errs)
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
// @ID           GetUpdateAvailableForDevice
// @Summary      Return list of available updates for a device.
// @Description  Return list of available updates for a device.
// @Tags         Devices (Systems)
// @Accept       json
// @Produce      json
// @Param        DeviceUUID    path     string     true   "DeviceUUID"
// @Param        latest        query    string     false  "query the latest or all updates"
// @Success      200 {object} models.Image
// @Failure      400 {object} errors.BadRequest "The request sent couldn't be processed."
// @Failure      500 {object} errors.InternalServerError "There was an internal server error."
// @Router       /updates/device/{DeviceUUID}/updates [get]
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
	result, _, err := contextServices.DeviceService.GetUpdateAvailableForDeviceByUUID(dc.DeviceUUID, latest, pagination.Limit, pagination.Offset)
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

// GetDeviceImageInfo returns the information of a running image
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
// @ID           GetDevice
// @Summary      Get a device by UUID.
// @Description  Get a device by UUID.
// @Tags         Devices (Systems)
// @Accept       json
// @Produce      json
// @Param        DeviceUUID   path     string     true   "DeviceUUID"
// @Success      200 {object} models.DeviceDetailsAPI
// @Failure      400 {object} errors.BadRequest "The request sent couldn't be processed."
// @Failure      404 {object} errors.NotFound "The device was not found."
// @Failure      500 {object} errors.InternalServerError "There was an internal server error."
// @Router       /devices/{DeviceUUID} [get]
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
// @ID           GetDevices
// @Summary      Get All Devices.
// @Description  Get combined system data from Edge API and Inventory API
// @Tags         Devices (Systems)
// @Accept       json
// @Produce      json
// @Param	 per_page        query int     false "field: maximum devices per page"
// @Param	 page            query int     false "field: which page to query from"
// @Param	 order_by        query string  false "field: order by display_name, updated or operating_system"
// @Param	 order_how       query string  false "field: choose to order ASC or DESC when order_by is being used"
// @Param	 hostname_or_id  query string  false "field: filter by hostname_or_id"
// @Success      200  {object}  models.DeviceDetailsListAPI
// @Failure      500 {object} errors.InternalServerError "There was an internal server error."
// @Router       /devices [get]
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
// @ID           GetDevicesView
// @Summary      Return all data of Devices.
// @Description  Return all data of Devices.
// @Tags         Devices (Systems)
// @Accept       json
// @Produce      json
// @Param	 sort_by            query string	false "fields: name, uuid, update_available, image_id. To sort DESC use - before the fields."
// @Param	 name               query string 	false "field: filter by name"
// @Param	 update_available   query boolean	false "field: filter by update_available"
// @Param	 uuid               query string	false "field: filter by uuid"
// @Param	 created_at         query string	false "field: filter by creation date"
// @Param	 image_id           query int   	false "field: filter by image id"
// @Param	 limit              query int    	false "field: return number of devices until limit is reached. Default is 100."
// @Param	 offset             query int    	false "field: return number of devices begining at the offset."
// @Success      200  {object}  models.DeviceViewListResponseAPI
// @Failure      500 {object} errors.InternalServerError "There was an internal server error."
// @Router       /devices/devicesview [get]
func GetDevicesView(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, contextServices.Log)
	if orgID == "" {
		return
	}
	tx := devicesFilters(r, db.DB).Where("image_id > 0").Session(&gorm.Session{})
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
	// set whether to enforce edge groups
	devicesViewList.EnforceEdgeGroups = utility.EnforceEdgeGroups(orgID)
	respondWithJSONBody(w, contextServices.Log, map[string]interface{}{"data": devicesViewList, "count": devicesCount})
}

// GetDevicesViewWithinDevices returns all data needed to display customers devices
// @ID           GetDevicesViewWithinDevices
// @Summary      Return all data of Devices.
// @Description  Return all data of Devices.
// @Tags         Devices (Systems)
// @Accept       json
// @Produce      json
// @Param    body	body	models.FilterByDevicesAPI true	"request body"
// @Param	 sort_by            query string	false "fields: name, uuid, update_available, image_id. To sort DESC use - before the fields."
// @Param	 name               query string 	false "field: filter by name"
// @Param	 update_available   query boolean	false "field: filter by update_available"
// @Param	 uuid               query string	false "field: filter by uuid"
// @Param	 created_at         query string	false "field: filter by creation date"
// @Param	 image_id           query int   	false "field: filter by image id"
// @Param	 limit              query int    	false "field: return number of devices until limit is reached. Default is 100."
// @Param	 offset             query int    	false "field: return number of devices beginning at the offset."
// @Success      200  {object}  models.DeviceViewListResponseAPI
// @Failure      500 {object} errors.InternalServerError "There was an internal server error."
// @Router       /devices/devicesview [post]
func GetDevicesViewWithinDevices(w http.ResponseWriter, r *http.Request) {
	contextServices := dependencies.ServicesFromContext(r.Context())
	orgID := readOrgID(w, r, contextServices.Log)
	if orgID == "" {
		return
	}

	var devicesUUID models.FilterByDevicesAPI
	if err := readRequestJSONBody(w, r, contextServices.Log, &devicesUUID); err != nil {
		return
	}
	if len(devicesUUID.DevicesUUID) == 0 {
		respondWithAPIError(w, contextServices.Log, errors.NewBadRequest("Missing devicesUUID "))
		return
	}

	tx := devicesFilters(r, db.DB).Where("image_id > 0").Where("devices.uuid IN (?)", devicesUUID.DevicesUUID).Session(&gorm.Session{})
	pagination := common.GetPagination(r)

	devicesCount, err := contextServices.DeviceService.GetDevicesCount(tx)
	if err != nil {
		respondWithAPIError(w, contextServices.Log, errors.NewInternalServerError())
		return
	}

	devicesViewList, err := contextServices.DeviceService.GetDevicesView(pagination.Limit, pagination.Offset, tx)
	if err != nil {
		respondWithAPIError(w, contextServices.Log, errors.NewInternalServerError())
		return
	}
	// set whether to enforce edge groups
	devicesViewList.EnforceEdgeGroups = utility.EnforceEdgeGroups(orgID)
	respondWithJSONBody(w, contextServices.Log, map[string]interface{}{"data": devicesViewList, "count": devicesCount})
}
