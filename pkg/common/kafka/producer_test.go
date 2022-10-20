// FIXME: golangci-lint
// nolint:revive
package kafkacommon_test

import (
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

var _ = Describe("Kafka Producer Test", func() {
	var mockTopicService *mock_kafkacommon.MockTopicServiceInterface
	var mockKafkaConfigService *mock_kafkacommon.MockKafkaConfigMapServiceInterface
	var service *kafkacommon.ProducerService
	var ctrl *gomock.Controller
	var kafkaConfigMap *kafka.ConfigMap

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)
		mockKafkaConfigService = mock_kafkacommon.NewMockKafkaConfigMapServiceInterface(ctrl)
		service = &kafkacommon.ProducerService{
			Topic:          mockTopicService,
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
		service.UnsetProducer()
	})
	Describe("Get Producer instance", func() {
		When("One does not exist yet", func() {
			Context("One does not exist yet", func() {
				It("should be ok", func() {
					mockKafkaConfigService.EXPECT().GetKafkaProducerConfigMap().Return(*kafkaConfigMap)
					p := service.GetProducerInstance()
					Expect(p).ToNot(BeNil())

				})
			})
			Context("Producer is a singleton", func() {
				It("should be the same producer", func() {
					mockKafkaConfigService.EXPECT().GetKafkaProducerConfigMap().Return(*kafkaConfigMap)
					p1 := service.GetProducerInstance()
					Expect(p1).ToNot(BeNil())

					p2 := service.GetProducerInstance()
					Expect(p2).ToNot(BeNil())
					Expect(p1).To(Equal(p2))

				})
			})
		})
	})
	Describe("Get Producer instance fails", func() {
		When("One dosent exist yet two", func() {
			Context("Kafka Config errors", func() {
				It("Producer should be nil", func() {
					kafkaConfigMap2 := kafka.ConfigMap{
						"bootstrap.servers": "192.168.1.2:80",
						"sasl.mechanisms":   gomock.Any(),
						"security.protocol": gomock.Any(),
					}
					mockKafkaConfigService.EXPECT().GetKafkaProducerConfigMap().Return(kafkaConfigMap2)
					p1 := service.GetProducerInstance()
					Expect(p1).To(BeNil())
				})
			})
		})
	})
	Describe("Produce Event", func() {
		When("OK", func() {
			Context("ok", func() {
				It("should be ok", func() {
					topic := "Test Topic"
					mockTopicService.EXPECT().GetTopic(gomock.Any()).Return(topic, nil)
					mockKafkaConfigService.EXPECT().GetKafkaProducerConfigMap().Return(nil)
					imageSet := &models.ImageSet{
						Name:    "test",
						Version: 2,
						OrgID:   common.DefaultOrgID,
					}
					image := &models.Image{
						Commit: &models.Commit{
							OSTreeCommit: faker.UUIDHyphenated(),
							OrgID:        common.DefaultOrgID,
						},
						Status:     models.ImageStatusSuccess,
						ImageSetID: &imageSet.ID,
						Version:    1,
						OrgID:      common.DefaultOrgID,
						Name:       gomock.Any().String(),
					}
					var ident identity.XRHID
					ident.Identity.OrgID = common.DefaultOrgID
					edgePayload := &models.EdgeImageRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      image.RequestID,
						},
						NewImage: *image,
					}

					cloudEvent := models.CRCCloudEvent{
						Data:        edgePayload,
						DataSchema:  "v1",
						ID:          faker.HyphenatedID,
						RedHatOrgID: common.DefaultOrgID,
						Source:      models.SourceEdgeEventAPI,
						SpecVersion: "v1",
						Subject:     image.Name,
						Time:        time.Now().Format(time.RFC3339),
						Type:        models.EventTypeEdgeImageRequested,
					}
					err := service.ProduceEvent(gomock.Any().String(), gomock.Any().String(), cloudEvent)
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
