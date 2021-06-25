package commits

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/redhatinsights/edge-api/pkg/common"
)

// MakeRouter adds support for operations on commits
func MakeRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", getDevices)

}

// getDevices registered for an account
func getDevices(w http.ResponseWriter, r *http.Request) {
	//	curl -X GET "https://qa.cloud.redhat.com/api/inventory/v1/hosts?per_page=1&page=1&staleness=fresh&staleness=stale&staleness=unknown" -H  "accept: application/json"
	account, err := common.GetAccount(r)
	pagination := common.GetPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

}

// example host:
// {
// 	"total":19732,
// 	"count":1,
// 	"page":1,
// 	"per_page":1,
// 	"results":[
// 	   {
// 		  "insights_id":"a32ec4d2-396a-4597-b903-fe117e597575",
// 		  "rhel_machine_id":null,
// 		  "subscription_manager_id":"b4698e0f-d6e2-4883-a1e0-f3155006b7bc",
// 		  "satellite_id":null,
// 		  "bios_uuid":"cba9942f-08bc-46f5-92a1-3450ab2614e0",
// 		  "ip_addresses":null,
// 		  "fqdn":"RHIQE.9c45db99-8db9-4642-b205-77b04beef430.srv-00.bishop.com",
// 		  "mac_addresses":null,
// 		  "external_id":null,
// 		  "provider_id":null,
// 		  "provider_type":null,
// 		  "id":"7b9936e4-02cd-42b9-a5f0-95eddae5b7c4",
// 		  "account":"6089719",
// 		  "display_name":"RHIQE.9c45db99-8db9-4642-b205-77b04beef430.srv-00.bishop.com",
// 		  "ansible_host":null,
// 		  "facts":[

// 		  ],
// 		  "reporter":"puptoo",
// 		  "stale_timestamp":"2021-06-26T03:29:55.986638+00:00",
// 		  "stale_warning_timestamp":"2021-07-03T03:29:55.986638+00:00",
// 		  "culled_timestamp":"2021-07-10T03:29:55.986638+00:00",
// 		  "created":"2021-06-24T22:29:56.089309+00:00",
// 		  "updated":"2021-06-24T22:29:56.089314+00:00"
// 	   }
// 	]
//  }
