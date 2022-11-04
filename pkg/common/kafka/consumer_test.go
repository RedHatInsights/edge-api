// FIXME: golangci-lint
// nolint:revive
package kafkacommon_test

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
)

var _ = Describe("Kafka Consumer Test", func() {
	var mockKafkaConfigService *mock_kafkacommon.MockKafkaConfigMapServiceInterface
	var ctrl *gomock.Controller
	var service *kafkacommon.ConsumerService
	var kafkaConfigMap *kafka.ConfigMap

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKafkaConfigService = mock_kafkacommon.NewMockKafkaConfigMapServiceInterface(ctrl)
		service = &kafkacommon.ConsumerService{
			KafkaConfigMap: mockKafkaConfigService,
		}
		config.Init()
		authType := clowder.BrokerConfigAuthtype("Auth")
		dummyString := gomock.Any().String()
		mech := "PLAIN"
		proto := "SASL_SSL"
		port := 80
		brokerConfig := clowder.BrokerConfig{
			Authtype: &authType,
			Cacert:   &dummyString,
			Hostname: "192.168.1.7",
			Port:     &port,
			Sasl: &clowder.KafkaSASLConfig{
				SaslMechanism:    &mech,
				SecurityProtocol: &proto,
				Username:         &dummyString,
				Password:         &dummyString,
			},
		}
		brokerSlice := []clowder.BrokerConfig{brokerConfig}
		config.Get().KafkaBrokers = brokerSlice
		kafkaConfigMap = &kafka.ConfigMap{
			"bootstrap.servers": "192.168.1.3:80",
		}

	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("Get Consumer", func() {
		When("does not exist yet", func() {
			Context("One does not exist yet", func() {
				FIt("should be ok", func() {

					consumerGroup := "imagesbuild"

					mockKafkaConfigService.EXPECT().GetKafkaConsumerConfigMap(consumerGroup).Return(*kafkaConfigMap)
					c, err := service.GetConsumer(consumerGroup)
					Expect(c).To(BeNil())
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
