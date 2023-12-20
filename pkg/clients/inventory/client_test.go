package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Inventory Client Tests", func() {
	var client inventory.ClientInterface
	var originalInventoryURL string
	conf := config.Get()
	BeforeEach(func() {
		client = inventory.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
		originalInventoryURL = conf.InventoryConfig.URL
	})
	AfterEach(func() {
		conf.ImageBuilderConfig.URL = originalInventoryURL
	})
	It("should init client", func() {
		Expect(client).ToNot(BeNil())
	})
	Context("ReturnDevicesByID", func() {

		It("should return inventory hosts by id successfully", func() {
			deviceUUID := faker.UUIDHyphenated()
			expectedParams := map[string]string{
				"filter[system_profile][host_type]": "edge",
				"fields[system_profile]": "host_type,operating_system,greenboot_status," +
					"greenboot_fallback_detected,rpm_ostree_deployments,rhc_client_id,rhc_config_state",
				"hostname_or_id": deviceUUID,
			}

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				urlQueryValues := r.URL.Query()
				for key, value := range expectedParams {
					Expect(value).To(Equal(urlQueryValues.Get(key)))
				}
				w.WriteHeader(http.StatusOK)
				response := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{{ID: deviceUUID}}}
				err := json.NewEncoder(w).Encode(&response)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()
			config.Get().InventoryConfig.URL = ts.URL
			result, err := client.ReturnDevicesByID(deviceUUID)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Total).To(Equal(1))
			Expect(result.Count).To(Equal(1))
			Expect(len(result.Result)).To(Equal(1))
			Expect(result.Result[0].ID).To(Equal(deviceUUID))
		})
	})
})
