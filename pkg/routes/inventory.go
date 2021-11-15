package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/services"
)

// MakeDevicesRouter adds support for operations on invetory
func MakeInventoryRouter(sub chi.Router) {
	sub.With(InventoyCtx).Get("/", GetInventory)
}

type InventoryData struct {
	Total    int
	Count    int
	Page     int
	Per_page int
	Results  []InventoryResponse
}

type InventoryResponse struct {
	ID         string
	DeviceName string
	LastSeen   string
	ImageInfo  *services.ImageInfo
}

type inventoryTypeKey int

const inventoryKey inventoryTypeKey = iota

func InventoyCtx(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var parameters inventory.InventoryParams

		fmt.Printf("UrlQuery::: %v\n", r.URL.Query())
		parameters.PerPage = r.URL.Query().Get("per_page")
		parameters.Page = r.URL.Query().Get("page")
		parameters.OrderBy = r.URL.Query().Get("order_by")
		parameters.OrderHow = r.URL.Query().Get("order_how")
		parameters.HostnameOrID = r.URL.Query().Get("hostname_or_id")
		parameters.DeviceStatus = r.URL.Query().Get("device_status")
		ctx := context.WithValue(r.Context(), inventoryKey, &parameters)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}
func GetInventory(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("entrei na rota\n")
	ctx := r.Context()
	fmt.Printf("ctx:: %v\n", ctx.Value(inventoryKey))
	param := ctx.Value(inventoryKey).(*inventory.InventoryParams)
	fmt.Printf("param:: %v\n", param)
	client := inventory.InitClient(ctx)
	var InventoryData InventoryData
	var results []InventoryResponse
	inventory, err := client.ReturnDevices(param)
	if err != nil || inventory.Count == 0 {
		err := errors.NewNotFound(fmt.Sprintf("No devices found "))
		w.WriteHeader(err.GetStatus())

	}

	results = GetUpdateAvailableInfo(r, inventory)

	InventoryData.Count = inventory.Count
	InventoryData.Total = inventory.Total
	InventoryData.Results = results

	json.NewEncoder(w).Encode(InventoryData)
}

func GetUpdateAvailableInfo(r *http.Request, inventoryResp inventory.Response) (IvtResponse []InventoryResponse) {
	var ivt []InventoryResponse
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	deviceService := services.DeviceService
	ctx := r.Context()
	param := ctx.Value(inventoryKey).(*inventory.InventoryParams)
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
		if param.DeviceStatus == "update_availabe" && imageInfo.UpdatesAvailable != nil {
			ivt = append(ivt, i)
		} else if param.DeviceStatus == "running" && imageInfo.UpdatesAvailable == nil {
			ivt = append(ivt, i)
		} else if param.DeviceStatus == "" {
			ivt = append(ivt, i)
		}
	}
	return ivt
}
