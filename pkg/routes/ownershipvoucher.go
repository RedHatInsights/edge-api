package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"

	"github.com/go-chi/chi"
)

// MakeFDORouter creates a router for the FDO API
func MakeFDORouter(sub chi.Router) {
	sub.Route("/ownership_voucher", func(r chi.Router) {
		r.Use(validateMiddleware)
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

	data, _ := ioutil.ReadAll(r.Body)

	numOfOVs := r.Header.Get("X-Number-Of-Vouchers")
	numOfOVsInt, _ := strconv.Atoi(numOfOVs) // checking for the error is done in the validation function

	resp, err := services.OwnershipVoucherService.BatchUploadOwnershipVouchers(data, uint(numOfOVsInt))
	if err != nil {
		services.Log.Error("Couldn't upload ownership vouchers ", err.Error())
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

	dataBytes, _ := ioutil.ReadAll(r.Body)
	data := []string{}
	err := json.Unmarshal(dataBytes, &data)
	if err != nil { // can't unmarshal json
		w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
		json.NewEncoder(w).Encode(err)
		return
	}

	resp, err := services.OwnershipVoucherService.BatchDeleteOwnershipVouchers(data)
	if err != nil {
		services.Log.Error("Couldn't delete ownership vouchers ", err.Error())
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
	if vNum, err := strconv.Atoi(r.Header.Get("X-Number-Of-Vouchers")); vNum < 0 || err != nil {
		return errors.NewBadRequest("X-Number-Of-Vouchers header must be set & greater than zero")
	}
	return nil
}

// validate delete request headers
func validateDeleteRequestHeaders(r *http.Request) errors.APIError {
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.NewBadRequest("Content-Type header must be application/json")
	}
	return nil
}

func validateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			w.WriteHeader(errors.NewBadRequest("Body is empty").GetStatus())
			json.NewEncoder(w).Encode(errors.NewBadRequest("Body is empty"))
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			w.WriteHeader(errors.NewBadRequest("Accept header must be set").GetStatus())
			json.NewEncoder(w).Encode(errors.NewBadRequest("Accept header must be set"))
			return
		}
		_, ok := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
		if !ok {
			w.WriteHeader(errors.NewInternalServerError().GetStatus())
			json.NewEncoder(w).Encode(interface{}("Internal server error"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
