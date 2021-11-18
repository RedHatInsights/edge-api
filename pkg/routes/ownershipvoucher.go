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

	validationErr := validateUploadRequestHeaders(r)
	if validationErr != nil {
		services.Log.Error("Couldn't validate ownership voucher upload request headers ", validationErr.Error())
		w.WriteHeader(validationErr.GetStatus())
		json.NewEncoder(w).Encode(validationErr)
		return
	}

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

	validationErr := validateDeleteRequestHeaders(r)
	if validationErr != nil {
		services.Log.Error("Couldn't validate ownership voucher delete request headers ", validationErr.Error())
		w.WriteHeader(validationErr.GetStatus())
		json.NewEncoder(w).Encode(validationErr)
		return
	}

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

// validate upload request headers
func validateUploadRequestHeaders(r *http.Request) errors.APIError {
	if r.Header.Get("Content-Type") != "application/cbor" {
		return errors.NewBadRequest("Content-Type header must be application/cbor")
	}
	if r.Header.Get("Accept") != "application/json" {
		return errors.NewBadRequest("Accept header must be set")
	}
	if vNum, err := strconv.Atoi(r.Header.Get("X-Number-Of-Vouchers")); vNum < 0 && err != nil {
		return errors.NewBadRequest("X-Number-Of-Vouchers header must be set & greater than zero")
	}
	return nil
}

// validate delete request headers
func validateDeleteRequestHeaders(r *http.Request) errors.APIError {
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.NewBadRequest("Content-Type header must be application/json")
	}
	if r.Header.Get("Accept") != "application/json" {
		return errors.NewBadRequest("Accept header must be set")
	}
	return nil
}
