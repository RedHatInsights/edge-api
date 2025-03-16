// FIXME: golangci-lint
// nolint:revive,typecheck
package kafkacommon_test

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/db"

	"github.com/redhatinsights/edge-api/config"
)

var _ = Describe("Kafka Consumer Test", func() {
	var mockKafkaConfigService *mock_kafkacommon.MockKafkaConfigMapServiceInterface
	var service *kafkacommon.ConsumerService
	var ctrl *gomock.Controller
	var kafkaConfigMap *kafka.ConfigMap

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKafkaConfigService = mock_kafkacommon.NewMockKafkaConfigMapServiceInterface(ctrl)
		service = &kafkacommon.ConsumerService{
			KafkaConfigMap: mockKafkaConfigService,
		}
		config.Init()
		cfg := config.Get()
		db.InitDB()

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
		cfg.KafkaBrokers = brokerSlice
		kafkaConfigMap = &kafka.ConfigMap{
			"bootstrap.servers": "192.168.39.1:80",
			"group.id":          "imagesisobuild",
		}
	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("Get Consumer", func() {
		When("One does not exist yet", func() {
			Context("One does not exist yet", func() {
				It("should be ok", func() {
					consumerGroupID := "imagesisobuild"
					mockKafkaConfigService.EXPECT().GetKafkaConsumerConfigMap(consumerGroupID).Return(*kafkaConfigMap)
					c, err := service.GetConsumer(consumerGroupID)
					Expect(err).To(BeNil())
					Expect(c).ToNot(BeNil())
				})
			})
		})
	})
})
