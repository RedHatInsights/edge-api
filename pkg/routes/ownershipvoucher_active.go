//go:build fdo
// +build fdo

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
		r.Use(validateMiddleware)                  // TODO: Consistent logging
		r.Post("/", CreateOwnershipVouchers)       // TODO: Consistent logging
		r.Post("/delete", DeleteOwnershipVouchers) // TODO: Consistent logging
		r.Post("/parse", ParseOwnershipVouchers)   // TODO: Consistent logging
		r.Post("/connect", ConnectDevices)         // TODO: Consistent logging
	})
}

// CreateOwnershipVouchers creates empty devices for the given ownership vouchers
func CreateOwnershipVouchers(w http.ResponseWriter, r *http.Request) {
	services, _ := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()

	validationErr := validateUploadRequestHeaders(r)
	if validationErr != nil {
		services.Log.Error("Couldn't validate ownership voucher upload request headers ", validationErr.Error())
		badRequestResponseBuilder(w, validationErr, "invalid_header")
		return
	}

	data, _ := ioutil.ReadAll(r.Body)

	numOfOVs := r.Header.Get("X-Number-Of-Vouchers")
	numOfOVsInt, _ := strconv.Atoi(numOfOVs) // checking for the error is done in the validation function

	resp, err := services.OwnershipVoucherService.BatchUploadOwnershipVouchers(data, uint(numOfOVsInt))
	if err != nil {
		switch err.Error() {
		case "bad request":
			services.Log.Error("Couldn't upload ownership vouchers ", err.Error())
			w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
			json.NewEncoder(w).Encode(resp)
			return
		default:
			services.Log.Error(err.Error())
			badRequestResponseBuilder(w, errors.NewBadRequest(err.Error()), "fdo_client")
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// DeleteOwnershipVouchers deletes devices for the given ownership vouchers GUIDs
func DeleteOwnershipVouchers(w http.ResponseWriter, r *http.Request) {
	services, _ := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()

	validationErr := validateContentTypeJSONHeader(r)
	if validationErr != nil {
		services.Log.Error("Couldn't validate ownership voucher delete request headers ", validationErr.Error())
		badRequestResponseBuilder(w, validationErr, "invalid_header")
		return
	}

	dataBytes, _ := ioutil.ReadAll(r.Body)
	data := []string{}
	err := json.Unmarshal(dataBytes, &data)
	if err != nil { // can't unmarshal json
		badRequestResponseBuilder(w, errors.NewBadRequest(err.Error()), "incomplete_body")
		return
	}

	resp, err := services.OwnershipVoucherService.BatchDeleteOwnershipVouchers(data)
	if err != nil {
		switch err.Error() {
		case "bad request":
			services.Log.Error("Couldn't delete ownership vouchers ", err.Error())
			w.WriteHeader(errors.NewBadRequest(err.Error()).GetStatus())
			json.NewEncoder(w).Encode(resp)
			return
		default:
			services.Log.Error(err.Error())
			badRequestResponseBuilder(w, errors.NewBadRequest(err.Error()), "fdo_client")
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ParseOwnershipVouchers parses ownership vouchers from the given cbor binary data
func ParseOwnershipVouchers(w http.ResponseWriter, r *http.Request) {
	services, _ := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()

	if r.Header.Get("Content-Type") != "application/cbor" {
		badRequestResponseBuilder(w, errors.NewBadRequest("Content-Type header must be application/cbor"), "invalid_header")
		return
	}
	data, _ := ioutil.ReadAll(r.Body)

	resp, err := services.OwnershipVoucherService.ParseOwnershipVouchers(data)
	if err != nil {
		services.Log.Error("Couldn't parse ownership vouchers ", err.Error())
		badRequestResponseBuilder(w, errors.NewBadRequest(err.Error()), "validation_parse_error")
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ConnectDevices connects devices to the given ownership vouchers
func ConnectDevices(w http.ResponseWriter, r *http.Request) {
	services, _ := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()

	validationErr := validateContentTypeJSONHeader(r)
	if validationErr != nil {
		services.Log.Error("Couldn't validate connect request headers ", validationErr.Error())
		badRequestResponseBuilder(w, validationErr, "invalid_header")
		return
	}

	dataBytes, _ := ioutil.ReadAll(r.Body)
	data := []string{}
	err := json.Unmarshal(dataBytes, &data)
	if err != nil { // can't unmarshal json
		badRequestResponseBuilder(w, errors.NewBadRequest(err.Error()), "incomplete_body")
		return
	}

	resp, errList := services.OwnershipVoucherService.ConnectDevices(data)
	if errList != nil {
		services.Log.Error("An error occured while trying to connect devices")
		var unknownDevices []string
		for _, err := range errList {
			unknownDevices = append(unknownDevices, err.Error())
		}
		w.WriteHeader(errors.NewBadRequest("unknown_device").GetStatus())
		resp := map[string]interface{}{"error_code": "unknown_device"}
		resp["error_details"] = map[string]interface{}{"unknown": unknownDevices}
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

// validate Content-Type application/json header
func validateContentTypeJSONHeader(r *http.Request) errors.APIError {
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.NewBadRequest("Content-Type header must be application/json")
	}
	return nil
}

func validateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			badRequestResponseBuilder(w, errors.NewBadRequest("Body is nil"), "incomplete_body")
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			badRequestResponseBuilder(w, errors.NewBadRequest("Accept header must be application/json"), "invalid_header")
			return
		}
		_, ok := dependencies.ServicesFromContext(r.Context())
		if !ok {
			w.WriteHeader(errors.NewInternalServerError().GetStatus())
			json.NewEncoder(w).Encode(interface{}("Internal server error"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// badRequestResponseBuilder builds a response for a bad request
func badRequestResponseBuilder(w http.ResponseWriter, e errors.APIError, errorCode string) {
	w.WriteHeader(e.GetStatus())
	resp := map[string]interface{}{"error_code": errorCode}
	resp["error_details"] = map[string]string{"error_message": e.Error()}
	json.NewEncoder(w).Encode(resp)
}
