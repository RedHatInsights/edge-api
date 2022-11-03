// FIXME: golangci-lint
// nolint:errcheck,govet,revive,typecheck
package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/image"
	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()
	// Init edge api services and attach them to the context
	edgeAPIServices := dependencies.Init(ctx)
	ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
	// create a base logger with fields to pass through the entire flow
	mslog := log.WithFields(log.Fields{"app": "edge", "service": "images"})

	mslog.Info("Microservice started")
	config.Init()
	cfg := config.Get()
	config.LogConfigAtStartup(cfg)

	db.InitDB()

	if cfg.KafkaConfig.Brokers == nil {
		mslog.WithField("error", "No kafka configuration found")
		os.Exit(1)
	}

	consumerGroup := "imagesisobuild"
	kafkaConfigMap := kafkacommon.NewKafkaConfigMapService().GetKafkaConsumerConfigMap(consumerGroup)
	c, err := kafka.NewConsumer(&kafkaConfigMap)

	if err != nil {
		mslog.WithField("error", err.Error()).Error("Failed to create ISO consumer")
		os.Exit(1)
	}

	mslog.WithField("consumer", c).Debug("Created ISO Consumer")
	topics := []string{kafkacommon.TopicFleetmgmtImageISOBuild}
	err = c.SubscribeTopics(topics, nil)
	if err != nil {
		mslog.Error("Subscribing to topics failed")
		os.Exit(1)
	}

	mslog.Info("ISO Microservice ready")

	run := true
	pollTime := 100
	for run {
		ev := c.Poll(pollTime)
		if ev == nil {
			continue
		}

		// handling event metadata
		switch e := ev.(type) {
		case *kafka.Message:
			key := string(e.Key)
			mslog = mslog.WithFields(log.Fields{
				"event_consumer_group": consumerGroup,
				"event_topic":          *e.TopicPartition.Topic,
				"event_partition":      e.TopicPartition.Partition,
				"event_offset":         e.TopicPartition.Offset,
				"event_recordkey":      string(e.Key),
			})
			mslog.WithField("message", string(e.Value)).Debug("Received an ISO event")
			if e.Headers != nil {
				mslog.WithField("headers", e.Headers).Debug("Headers received with the event")
			}

			switch key {
			case models.EventTypeEdgeImageISORequested:
				crcEvent := &image.EventImageISORequestedBuildHandler{}

				err = json.Unmarshal(e.Value, crcEvent)
				if err != nil {
					mslog.Error("Failed to unmarshal CRC ISO event")
					break
				}

				mslog = mslog.WithField("event_id", crcEvent.ID)
				ctx = image.ContextWithLogger(ctx, mslog)

				// call the event's Consume method
				go crcEvent.Consume(ctx)
			default:
				mslog.Trace("Record key is not recognized by ISO consumer: " + key)
			}

			// commit the Kafka offset
			_, err := c.Commit()
			if err != nil {
				mslog.WithField("error", err).Error("Error storing offset after ISO message")
			}
		case kafka.Error:
			// terminate the application if all brokers are down.
			log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting ISO loop due to Kafka broker issue")
			if e.Code() == kafka.ErrAllBrokersDown {
				run = false
			}
		default:
			log.WithField("event", e).Warning("Event ignored")
		}
	}
}
