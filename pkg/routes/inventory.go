package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
)

// MakeDevicesRouter adds support for operations on invetory
func MakeInventoryRouter(sub chi.Router) {
	sub.Get("/", GetInventory)
}

type InventoryData struct {
	Total    int
	Count    int
	Page     int
	Per_page int
	Results  []InventoryResponse
}

type InventoryResponse struct {
	ID              string
	DeviceName      string
	LastSeen        string
	UpdateAvailable bool
}

func GetInventory(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("entrei na rota/n")
	ctx := r.Context()
	client := inventory.InitClient(ctx)
	var InventoryData InventoryData
	var results []InventoryResponse
	inventory, err := client.ReturnDevices()
	if err != nil || inventory.Count == 0 {
		err := errors.NewNotFound(fmt.Sprintf("No devices found "))
		w.WriteHeader(err.GetStatus())

	}

	results = getUpdateAvailableInfo(r, inventory)

	InventoryData.Count = inventory.Count
	InventoryData.Total = inventory.Total
	InventoryData.Results = results

	json.NewEncoder(w).Encode(InventoryData)
}

func getUpdateAvailableInfo(r *http.Request, inventory inventory.Response) (IvtResponse []InventoryResponse) {
	var ivt []InventoryResponse
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	deviceService := services.DeviceService

	for _, device := range inventory.Result {
		var i InventoryResponse
		updateAvailable, err := deviceService.GetUpdateAvailableForDeviceByUUID(device.ID)
		i.ID = device.ID
		i.DeviceName = device.DisplayName
		i.LastSeen = device.LastSeen

		if err != nil {
			i.UpdateAvailable = false

		} else if updateAvailable != nil {
			i.UpdateAvailable = true
		}
		ivt = append(ivt, i)
	}
	return ivt
}
