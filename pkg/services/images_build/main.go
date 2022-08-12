package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/images"
	log "github.com/sirupsen/logrus"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"

	l "github.com/redhatinsights/edge-api/logger" // is this one really needed with logrus?
	"github.com/redhatinsights/edge-api/pkg/db"
)

func main() {
	// create a new context
	ctx := context.Background()
	// create a base logger with fields to pass through the entire flow
	mslog := log.WithFields(log.Fields{"app": "edge", "service": "images"})

	mslog.Info("Microservice started")

	// FIXME: a good opportunity to refactor config
	config.Init()
	l.InitLogger()
	cfg := config.Get()
	// TODO: update these fields
	mslog.WithFields(log.Fields{
		"Hostname":                 cfg.Hostname,
		"Auth":                     cfg.Auth,
		"WebPort":                  cfg.WebPort,
		"MetricsPort":              cfg.MetricsPort,
		"LogLevel":                 cfg.LogLevel,
		"Debug":                    cfg.Debug,
		"BucketName":               cfg.BucketName,
		"BucketRegion":             cfg.BucketRegion,
		"RepoTempPath ":            cfg.RepoTempPath,
		"OpenAPIFilePath ":         cfg.OpenAPIFilePath,
		"ImageBuilderURL":          cfg.ImageBuilderConfig.URL,
		"InventoryURL":             cfg.InventoryConfig.URL,
		"PlaybookDispatcherConfig": cfg.PlaybookDispatcherConfig.URL,
		"TemplatesPath":            cfg.TemplatesPath,
		"DatabaseType":             cfg.Database.Type,
		"DatabaseName":             cfg.Database.Name,
		"EdgeAPIURL":               cfg.EdgeAPIBaseURL,
		"EdgeAPIServiceHost":       cfg.EdgeAPIServiceHost,
		"EdgeAPIServicePort":       cfg.EdgeAPIServicePort,
	}).Info("Configuration Values:")
	db.InitDB()

	if cfg.KafkaConfig.Brokers != nil {
		consumerGroup := "imagesbuild"

		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

		// TODO: this should be a struct defined elsewhere and read in
		c, err := kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers":     fmt.Sprintf("%s:%v", cfg.KafkaBrokers[0].Hostname, *cfg.KafkaBrokers[0].Port),
			"broker.address.family": "v4",
			"group.id":              consumerGroup,
			"session.timeout.ms":    6000,
			"auto.offset.reset":     "earliest",
			//			"enable.auto.offset.store": false,
		})

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
				time.Sleep(5)
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
						"consumer_group": consumerGroup,
						"topic":          *e.TopicPartition.Topic,
						"partition":      e.TopicPartition.Partition,
						"offset":         e.TopicPartition.Offset,
						"key":            string(e.Key),
					})
					mslog.WithField("message", string(e.Value)).Debug("Received an event")

					if e.Headers != nil {
						mslog.WithField("headers", e.Headers).Debug("Headers received with the event")
					}

					// route to specific event handler based on the event key
					mslog.Debug("consumer is routing based on record key")

					switch key {
					case models.EventTypeEdgeImageRequested:
						// TODO: this seems like a lot of work to read an event back in
						//			make this configurable and reusable
						// unmarshal the event bytes to a specific struct
						crcEvent := &images.ImageRequestedEvent{}
						err = json.Unmarshal(e.Value, crcEvent)
						if err != nil {
							mslog.Error("Failed to unmarshal CRC event")
						}

						// add event UUID to logger
						mslog = mslog.WithField("event_id", crcEvent.ID)

						// CRC Event standard inlines the payload in Data field
						//	we have to do some cast magic, but...
						//	because of how Unmarshal handles interfaces, it's easier to go to bytes and back
						edgePayloadString, err := json.Marshal(crcEvent.Data)
						if err != nil {
							mslog.Error("Failed to marshal Payload")
						}
						edgePayload := &models.EdgeImageRequestedEventPayload{}
						err = json.Unmarshal(edgePayloadString, edgePayload)
						if err != nil {
							mslog.Error("Failed to unmarshal Payload")
						}
						// add the correct struct implementation for payload back to CRC struct in Data
						crcEvent.Data = *edgePayload

						// TODO: drop this and the crcEvent.Consume() call below switch to fallthrough if event is of type with Consume().
						//		then set a flag and if it
						// add the logger to the context before Consume() calls
						ctx = images.ContextWithLogger(ctx, mslog)

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

// FIXME: move consumer config map to central location so consumer code doesn't need to be touched
