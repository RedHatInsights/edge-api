package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/services"
)

// MakeInventoryRouter adds support for operations on inventory
func MakeInventoryRouter(sub chi.Router) {
	sub.Get("/", GetInventory)
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
	ImageInfo  *services.ImageInfo
}

// GetInventory make the call to inventory api and inject edge info
func GetInventory(w http.ResponseWriter, r *http.Request) {
	var param *inventory.Params = new(inventory.Params)

	param.PerPage = r.URL.Query().Get("per_page")
	param.Page = r.URL.Query().Get("page")
	param.OrderBy = r.URL.Query().Get("order_by")
	param.OrderHow = r.URL.Query().Get("order_how")
	param.HostnameOrID = r.URL.Query().Get("hostname_or_id")
	param.DeviceStatus = r.URL.Query().Get("device_status")

	client := inventory.InitClient(r.Context())

	var InventoryData InventoryData
	var results []InventoryResponse

	inventory, err := client.ReturnDevices(param)
	if err != nil || inventory.Count == 0 {
		err := errors.NewNotFound("No devices found")
		w.WriteHeader(err.GetStatus())

	}
	fmt.Printf(":: inventory :: %v\n", inventory)
	results = GetUpdateAvailableInfo(param, r, inventory)

	fmt.Printf(":: inventory :: %v\n", results)
	InventoryData.Count = inventory.Count
	InventoryData.Total = inventory.Total
	InventoryData.Results = results

	json.NewEncoder(w).Encode(InventoryData)
}

// GetUpdateAvailableInfo returns the image information
func GetUpdateAvailableInfo(param *inventory.Params, r *http.Request, inventoryResp inventory.Response) (IvtResponse []InventoryResponse) {
	var ivt []InventoryResponse
	services, _ := dependencies.ServicesFromContext(r.Context())
	deviceService := services.DeviceService

	for _, device := range inventoryResp.Result {
		var i InventoryResponse
		imageInfo, err := deviceService.GetDeviceImageInfo(device.ID)
		i.ID = device.ID
		i.DeviceName = device.DisplayName
		i.LastSeen = device.LastSeen

		if err != nil {
			i.ImageInfo = nil

		} else if imageInfo != nil {
			i.ImageInfo = imageInfo
		}
		if param != nil && param.DeviceStatus == "update_available" && imageInfo.UpdatesAvailable != nil {
			ivt = append(ivt, i)
		} else if param != nil && param.DeviceStatus == "running" && imageInfo.UpdatesAvailable == nil {
			ivt = append(ivt, i)
		} else if param != nil && param.DeviceStatus == "" {
			ivt = append(ivt, i)
		} else {
			ivt = append(ivt, i)
		}
	}
	return ivt
}
