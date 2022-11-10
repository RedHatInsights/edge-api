package main

import (
	"context"
	"encoding/json"
	eventImageReq "github.com/redhatinsights/edge-api/pkg/services/image"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

var _ = Describe("Image Iso Kafka Consumer Test", func() {
	var ctx context.Context
	var mockKafkaConfigService *mock_kafkacommon.MockKafkaConfigMapServiceInterface
	var mockConsumerService *mock_kafkacommon.MockConsumerServiceInterface
	var mockConsumer *mock_kafkacommon.MockConsumer
	var ctrl *gomock.Controller
	var kafkaConfigMap *kafka.ConfigMap

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())
		ctx = eventImageReq.ContextWithLogger(ctx, log.NewEntry(log.StandardLogger()))
		mockConsumerService = mock_kafkacommon.NewMockConsumerServiceInterface(ctrl)
		mockConsumer = mock_kafkacommon.NewMockConsumer(ctrl)
		mockKafkaConfigService = mock_kafkacommon.NewMockKafkaConfigMapServiceInterface(ctrl)
		config.Init()
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
		kafkaConfig := clowder.KafkaConfig{Brokers: brokerSlice}
		config.Get().KafkaConfig = &kafkaConfig
		kafkaConfigMap = &kafka.ConfigMap{
			"bootstrap.servers": "192.168.1.3:80",
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
					timeout := 100
					topic := "platform.edge.fleetmgmt.image-iso-build"
					mockKafkaConfigService.EXPECT().GetKafkaConsumerConfigMap(consumerGroupID).Return(*kafkaConfigMap)
					mockConsumerService.EXPECT().GetConsumer(consumerGroupID).Return(mockConsumer, nil)
					mockConsumer.EXPECT().Subscribe(topic, nil)

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

					recordKey := "com.redhat.console.edge.api.image.requested"
					key1 := []byte(recordKey)
					message, err := json.Marshal(cloudEvent)
					kafkaMessage := kafka.Message{
						Key:   key1,
						Value: message}

					mockConsumer.EXPECT().Poll(timeout).Return(&kafkaMessage)
					main()
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
