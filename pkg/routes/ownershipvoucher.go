package routes

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fxamacker/cbor/v2"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"

	"github.com/go-chi/chi"
)

// MakeFDORouter creates a router for the FDO API
func MakeFDORouter(sub chi.Router) {
	sub.Route("/ownership_voucher", func(r chi.Router) {
		r.Post("/", CreateOwnershipVouchers)
		r.Post("/delete", DeleteOwnershipVouchers)
	})
}

// CreateOwnershipVouchers creates empty devices for the given ownership vouchers
func CreateOwnershipVouchers(w http.ResponseWriter, r *http.Request) {
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	defer r.Body.Close()

	decoder := cbor.NewDecoder(r.Body)
	data := []byte{}
	err := decoder.Decode(&data)

	if err != nil { // bad CBOR body
		services.Log.Error("Couldn't marshal body into CBOR ", err)
		w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
		json.NewEncoder(w).Encode(err)
		return
	}
	numOfOVs := r.Header.Get("X-Number-Of-Vouchers")
	numOfOVsInt, _ := strconv.Atoi(numOfOVs)

	resp, err := services.OwnershipVoucherService.BatchUploadOwnershipVouchers(data, uint(numOfOVsInt))
	if err != nil {
		services.Log.Error("Couldn't upload ownership vouchers ", err)
		w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
		json.NewEncoder(w).Encode(resp)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// DeleteOwnershipVouchers deletes devices for the given ownership vouchers GUIDs
func DeleteOwnershipVouchers(w http.ResponseWriter, r *http.Request) {
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	data := []string{}
	err := decoder.Decode(&data)

	if err != nil {
		services.Log.Error("Couldn't parse JSON body ", err)
		w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
		json.NewEncoder(w).Encode(err)
		return
	}
	resp, err := services.OwnershipVoucherService.BatchDeleteOwnershipVouchers(data)
	if err != nil {
		services.Log.Error("Couldn't delete ownership vouchers ", err)
		w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
		json.NewEncoder(w).Encode(resp)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
