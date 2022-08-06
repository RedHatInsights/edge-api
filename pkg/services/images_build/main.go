package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	log "github.com/sirupsen/logrus"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"

	l "github.com/redhatinsights/edge-api/logger" // is this one really needed with logrus?
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/services/images"
)

func main() {
	log.WithField("microservice", "images-build").Info("Microservice started")

	// FIXME: a good opportunity to refactor config
	config.Init()
	l.InitLogger()
	cfg := config.Get()
	// TODO: update these fields
	log.WithFields(log.Fields{
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
			log.WithField("error", err).Error("Failed to create consumer")
			os.Exit(1)
		}

		log.WithField("consumer", c).Debug("Created Consumer")

		// TODO: define this by mapping topics to a microservice struct
		// TODO: and nail record keys to the topic
		// TODO: make this main.go a single run engine for all microservices
		topics := []string{kafkacommon.TopicFleetmgmtImageBuild}
		err = c.SubscribeTopics(topics, nil)
		if err != nil {
			log.Error("Subscribing to topics failed")
		}

		run := true

		for run {
			select {
			case sig := <-sigchan:
				log.WithField("signal", sig).Debug("Caught signal and terminating")
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
					logevent := log.WithFields(log.Fields{
						"topic":     *e.TopicPartition.Topic,
						"partition": e.TopicPartition.Partition,
						"offset":    e.TopicPartition.Offset,
						"key":       string(e.Key)})
					logevent.WithField("message", string(e.Value)).Debug("Received an event")

					if e.Headers != nil {
						logevent.WithField("headers", e.Headers).Debug("Headers received with the event")
					}

					// route to specific event handler based on the event key
					logevent.Debug("consumer routing based on key")

					// execute the event handler if the record key has been defined
					if _, exists := images.RegisteredEvents[key]; exists {
						edgeEvent := images.RegisteredEvents[key]
						json.Unmarshal(e.Value, edgeEvent)
						// using reflection to avoid the compiler error with Consume() and an unknown struct
						go reflect.ValueOf(edgeEvent).MethodByName("Consume").Call(nil)
					} else {
						logevent.Warning("Skipping event. Record key is not defined")
					}

					_, err := c.Commit()
					if err != nil {
						logevent.WithField("error", err).Error("Error storing offset after message")
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
