// FIXME: golangci-lint
// nolint:govet,revive,typecheck
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/image"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

func initConsumerImageBuild(ctx context.Context) {
	edgeAPIServices := dependencies.ServicesFromContext(ctx)
	mslog := log.WithFields(log.Fields{"app": "edge", "service": "images"})

	mslog.Info("Microservice started")
	cfg := config.Get()
	config.LogConfigAtStartup(cfg)

	db.InitDB()

	if cfg.KafkaConfig.Brokers != nil {
		consumerGroup := "imagesbuild"

		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

		c, err := edgeAPIServices.ConsumerService.GetConsumer(consumerGroup)

		if err != nil {
			mslog.WithField("error", err.Error()).Error("Failed to create consumer")
			os.Exit(1)
		}

		mslog.WithField("consumer", c).Debug("Created Consumer")

		// TODO: define this by mapping topics to a microservice struct
		// TODO: and nail record keys to the topic
		// TODO: make this main.go a single run engine for all microservices
		topics := []string{kafkacommon.TopicFleetmgmtImageBuild}
		err = c.SubscribeTopics(topics, nil)
		if err != nil {
			mslog.Error("Subscribing to topics failed")
			// TODO: handle retries
			// TODO: handle notifications
		}

		mslog.Info("Microservice ready")

		run := true
		for run {
			select {
			case sig := <-sigchan:
				mslog.WithField("signal", sig).Debug("Caught signal and terminating")
				sleepTime := time.Duration(5)
				time.Sleep(sleepTime)
				run = false
			default:
				ev := c.Poll(100)
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
					mslog.WithField("message", string(e.Value)).Debug("Received an event")

					if e.Headers != nil {
						mslog.WithField("headers", e.Headers).Debug("Headers received with the event")
					}

					// route to specific event handler based on the event key
					mslog.Debug("consumer is routing based on record key")

					switch key {
					case models.EventTypeEdgeImageRequested:
						crcEvent := &image.EventImageRequestedBuildHandler{}

						err = json.Unmarshal(e.Value, crcEvent)
						if err != nil {
							mslog.Error("Failed to unmarshal CRC event")
							break
						}

						// add event UUID to logger
						mslog = mslog.WithField("event_id", crcEvent.ID)

						// add the logger to the context before Consume() calls
						ctx = utility.ContextWithLogger(ctx, mslog)

						// call the event's Consume method
						imageService := services.NewImageService(ctx, mslog)
						go crcEvent.Consume(ctx, imageService)
					case models.EventTypeEdgeImageUpdateRequested:
						crcEvent := &image.EventImageUpdateRequestedBuildHandler{}
						err = json.Unmarshal(e.Value, crcEvent)
						if err != nil {
							mslog.Error("Failed to unmarshal CRC event")
						}

						// add event UUID to logger
						mslog = mslog.WithField("event_id", crcEvent.ID)

						// add the logger to the context before Consume() calls
						ctx = utility.ContextWithLogger(ctx, mslog)

						// call the event's Consume method
						go crcEvent.Consume(ctx)
					default:
						mslog.Trace("Record key is not recognized by consumer")
					}

					// commit the Kafka offset
					_, err := c.Commit()
					if err != nil {
						mslog.WithField("error", err).Error("Error storing offset after message")
					}
				case kafka.Error:
					// terminate the application if all brokers are down.
					log.WithFields(log.Fields{"code": e.Code(), "error": e}).Error("Exiting loop due to Kafka broker issue")
					if e.Code() == kafka.ErrAllBrokersDown {
						run = false
					}
				default:
					log.WithField("event", e).Warning("Event ignored")
				}
			}
		}
		log.Info("Closing consumer\n")
		c.Close()
	}

}

func main() {
	ctx := context.Background()
	edgeAPIServices := dependencies.Init(ctx)
	ctx = dependencies.ContextWithServices(ctx, edgeAPIServices)
	mslog := log.WithFields(log.Fields{"app": "edge", "service": "images"})
	ctx = utility.ContextWithLogger(ctx, mslog)
	initConsumerImageBuild(ctx)
}
