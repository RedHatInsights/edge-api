// FIXME: golangci-lint
// nolint:revive
package services_test

import (
	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/pkg/services"
)

var _ = Describe("ConsumerService basic functions", func() {
	Describe("creation of the service", func() {
		port := 9092
		config := &v1.KafkaConfig{Brokers: []v1.BrokerConfig{{Hostname: "localhost", Port: &port}}}
		topics := []string{"platform.playbook-dispatcher.runs", "platform.inventory.events"}
		nonRealTopic := faker.DomainName()
		Context("returns a correct instance", func() {
			for _, topic := range topics {
				s := services.NewKafkaConsumerService(config, topic)
				It("not to be nil", func() {
					Expect(s).ToNot(BeNil())
				})
			}
		})
		Context("nil instance", func() {
			for _, topic := range topics {
				It("returns nil", func() {
					s := services.NewKafkaConsumerService(nil, topic)
					Expect(s).To(BeNil())
				})
			}
			It("returns nil", func() {
				s := services.NewKafkaConsumerService(config, nonRealTopic)
				Expect(s).To(BeNil())
			})
		})
	})
})
