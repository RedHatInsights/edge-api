package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

// NOTE: this is currently designed for a single ibvents replica

func main() {
	// set things up
	log.Info("Starting up...")

	var images []models.Image
	// IBevent represents the struct of the value in a Kafka message
	// TODO: add the original requestid
	type IBevent struct {
		ImageID uint `json:"image_id"`
	}

	config.Init()
	l.InitLogger()
	cfg := config.Get()
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
		"DefaultOSTreeRef":         cfg.DefaultOSTreeRef,
		"InventoryURL":             cfg.InventoryConfig.URL,
		"PlaybookDispatcherConfig": cfg.PlaybookDispatcherConfig.URL,
		"TemplatesPath":            cfg.TemplatesPath,
		"DatabaseType":             cfg.Database.Type,
		"DatabaseName":             cfg.Database.Name,
	}).Info("Configuration Values:")
	db.InitDB()

	log.Info("Entering the infinite loop...")
	for {
		log.Debug("Sleeping...")
		time.Sleep(5 * time.Minute)
		// TODO: work out how to avoid resuming a build until app is up or on way up

		// check the database for image builds in INTERRUPTED status
		db.DB.Debug().Where(&models.Image{Status: models.ImageStatusInterrupted}).Find(&images)

		for _, image := range images {
			log.WithField("imageID", image.ID).Info("Found image with interrupted status")

			/* we have a choice here...
			1. Send an event and a consumer on Edge API calls the resume.
			2. Send an API call to Edge API to call the resume.

			Currently...
			1. Testing a Kafka event.
			2. Will implement a call to the API restart()
			3. Will create an API endpoint specifically for resume()
				so it can pick up where it left off
			*/

			if clowder.IsClowderEnabled() {
				// get the list of brokers from the config
				brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))
				for i, b := range clowder.LoadedConfig.Kafka.Brokers {
					brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
					fmt.Println(brokers[i])
				}

				topic := "platform.edge.fleetmgmt.image-build"

				// Create Producer instance
				// TODO: do this once before loop
				p, err := kafka.NewProducer(&kafka.ConfigMap{
					"bootstrap.servers": brokers[0]})
				if err != nil {
					log.WithField("error", err).Error("Failed to create producer")
				}
				// assemble the message to be sent
				// TODO: formalize message formats
				recordKey := "resume_image"
				ibvent := IBevent{}
				ibvent.ImageID = image.ID
				ibventMessage, _ := json.Marshal(ibvent)
				log.WithField("message", ibvent).Debug("Preparing record for producer")
				// send the message
				perr := p.Produce(&kafka.Message{
					TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
					Key:            []byte(recordKey),
					Value:          ibventMessage,
				}, nil)
				if perr != nil {
					log.Error("Error sending message")
				}

				// Wait for all messages to be delivered
				p.Flush(15 * 1000)

				// TODO: do this once at break from loop
				p.Close()

				log.WithField("topic", topic).Debug("IBvents interrupted build message was produced to topic")
			}
		}
	}

	// TODO: catch interrupts to note a SIGTERM was sent versus a crash/panic
}
