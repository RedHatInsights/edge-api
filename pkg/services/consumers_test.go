package services_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/pkg/services"
)

var _ = Describe("ConsumerService basic functions", func() {
	Describe("creation of the service", func() {
		Context("returns a correct instance", func() {
			p := 9092
			config := &v1.KafkaConfig{Brokers: []v1.BrokerConfig{{Hostname: "localhost", Port: &p}}}
			s := services.NewKafkaConsumerService(config)
			It("not to be nil", func() {
				Expect(s).ToNot(BeNil())
			})
		})
	})
})
