package routes

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fxamacker/cbor/v2"

	"github.com/redhatinsights/edge-api/pkg/dependencies"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func MakeFDORouter(sub chi.Router) {
	sub.Post("/ownership_voucher", CreateEmptyDevices)
	sub.Post("/ownership_voucher/delete", DeleteDevices)
}

func CreateEmptyDevices(w http.ResponseWriter, r *http.Request) {
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	bodyAsBytes, err := cbor.Marshal(r.Body)
	defer r.Body.Close()
	if err != nil { // bad CBOR body
		services.Log.Error("Couldn't marshal body into CBOR ", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(err)
		return
	}
	data, err := services.OwnershipVoucherService.ParseVouchers(bodyAsBytes)
	if err != nil { // couldn't parse vouchers
		services.Log.Error("Couldn't parse ownership vouchers ", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(err)
		return
	}
	numOfOVs := r.Header.Get("X-Number-Of-Vouchers")
	numOfOVsInt, _ := strconv.Atoi(numOfOVs)
	fdoClient := services.OwnershipVoucherService.CreateFDOClient()
	resp, err := fdoClient.BatchUpload(bodyAsBytes, uint(numOfOVsInt))
	if err != nil { // couldn't upload to FDO onboarding server
		services.Log.Error("Couldn't upload ownership vouchers ", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
	for _, voucher := range data {
		var device *models.Device
		device.UUID = voucher.GUID // make it searchable
		device.Connected = false
		device.FDO = &voucher
		db.DB.Save(&device)
	}
	return
}

func DeleteDevices(w http.ResponseWriter, r *http.Request) {
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	bodyAsBytes, err := json.Marshal(r.Body)
	defer r.Body.Close()
	if err != nil { // bad JSON body
		services.Log.Error("Couldn't marshal body into JSON ", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(err)
		return
	}
	fdoClient := services.OwnershipVoucherService.CreateFDOClient()
	data := []string{}
	err = json.Unmarshal(bodyAsBytes, &data)
	if err != nil {
		services.Log.Error("Couldn't parse JSON body ", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(err)
		return
	}
	resp, err := fdoClient.BatchDelete(data)
	if err != nil { // couldn't upload to FDO onboarding server
		services.Log.Error("Couldn't delete ownership vouchers ", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}
	for _, guid := range data {
		device, err := services.DeviceService.GetDeviceByUUID(guid)
		if err != nil {
			services.Log.Error("Couldn't find device ", guid, err)
			break
		}
		device.Connected = true
		db.DB.Save(&device)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
	return
}
