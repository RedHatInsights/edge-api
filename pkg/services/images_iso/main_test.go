package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

var _ = Describe("Image Iso Kafka Consumer Test", func() {
	var ctx context.Context
	var ctrl *gomock.Controller
	var mockImageService *mock_services.MockImageServiceInterface
	var mockConsumerService *mock_kafkacommon.MockConsumerServiceInterface
	var mockConsumer *mock_kafkacommon.MockConsumer

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())
		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
		mockConsumerService = mock_kafkacommon.NewMockConsumerServiceInterface(ctrl)
		mockConsumer = mock_kafkacommon.NewMockConsumer(ctrl)
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ImageService:    mockImageService,
			ConsumerService: mockConsumerService,
		})
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
	})

	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("Get Consumer", func() {
		When("One does not exist yet", func() {
			Context("One does not exist yet", func() {
				It("should be ok", func() {
					var ident identity.XRHID
					consumerGroupID := "imagesisobuild"
					timeout := 100
					ident.Identity.OrgID = common.DefaultOrgID
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
					recordKey := "com.redhat.console.edge.api.image.iso.requested"
					key1 := []byte(recordKey)
					message, err := json.Marshal(cloudEvent)
					Expect(err).To(BeNil())
					off := kafka.Offset(64)
					tp := kafka.TopicPartition{
						Topic:     &recordKey,
						Partition: 1,
						Offset:    off,
					}
					kafkaMessage := kafka.Message{
						Key:            key1,
						Value:          message,
						TopicPartition: tp,
					}
					mockConsumerService.EXPECT().GetConsumer(consumerGroupID).Return(mockConsumer, nil)
					mockConsumer.EXPECT().SubscribeTopics(gomock.Any(), gomock.Any())
					mockConsumer.EXPECT().Poll(timeout).Return(&kafkaMessage)
					mockConsumer.EXPECT().Commit().AnyTimes()
					kafkaError := kafka.NewError(kafka.ErrAllBrokersDown, "Error", true)
					mockConsumer.EXPECT().Poll(timeout).Return(&kafkaError).AnyTimes()
					initConsumer(ctx)
				})
			})
		})
	})
})
