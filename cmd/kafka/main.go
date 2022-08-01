package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

// NOTE: this is currently designed for a single ibvents replica

// LoopTime interrupt query loop sleep time in minutes
const LoopTime = 5

// get images with a build of x status and older than y hours
func getStaleBuilds(status string, age int) []models.Image {
	var images []models.Image

	// looks like we ran into a known pgx issue when using ? for parameters in certain prepared SQL statements
	// 		using Sprintf to predefine the query and pass to Where
	query := fmt.Sprintf("status = '%s' AND updated_at < NOW() - INTERVAL '%d hours'", status, age)
	qresult := db.DB.Debug().Where(query).Find(&images)
	if qresult.Error != nil {
		log.WithField("error", qresult.Error.Error()).Error("Stale builds query failed")
		return nil
	}

	if qresult.RowsAffected > 0 {
		log.WithFields(log.Fields{
			"numImages": qresult.RowsAffected,
			"status":    status,
			"interval":  age,
		}).Debug("Found stale image(s) with interval")
	}

	return images
}

// set the status for a specific image
func setImageStatus(id uint, status string) error {
	tx := db.DB.Debug().Model(&models.Image{}).Where("ID = ?", id).Update("Status", status)
	if tx.Error != nil {
		log.WithField("error", tx.Error.Error()).Error("Error updating image status")
		return tx.Error
	}

	log.WithField("imageID", id).Debug("Image updated with " + fmt.Sprint(status) + " status")

	return nil
}

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

	log.Info("Entering the infinite loop...")
	for {
		log.Debug("Sleeping...")
		// TODO: make this configurable
		time.Sleep(LoopTime * time.Minute)
		// TODO: programatic method to avoid resuming a build until app is up or on way up???

		// handle stale interrupted builds not complete after x hours
		staleInterruptedImages := getStaleBuilds(models.ImageStatusInterrupted, 6)
		for _, staleImage := range staleInterruptedImages {
			log.WithFields(log.Fields{
				"UpdatedAt": staleImage.UpdatedAt,
				"ID":        staleImage.ID,
				"Status":    staleImage.Status,
			}).Info("Processing stale interrupted image")

			statusUpdateError := setImageStatus(staleImage.ID, models.ImageStatusError)
			if statusUpdateError != nil {
				log.Error("Failed to update stale interrupted image build status")
			}
		}

		// handle stale builds not complete after x hours
		staleBuildingImages := getStaleBuilds(models.ImageStatusBuilding, 3)
		for _, staleImage := range staleBuildingImages {
			log.WithFields(log.Fields{
				"UpdatedAt": staleImage.UpdatedAt.Time.Local().String(),
				"ID":        staleImage.ID,
				"Status":    staleImage.Status,
			}).Info("Processing stale building image")

			statusUpdateError := setImageStatus(staleImage.ID, models.ImageStatusError)
			if statusUpdateError != nil {
				log.Error("Failed to update stale building image build status")
			}
		}

		// handle image builds in INTERRUPTED status
		//	this is meant to handle builds that are interrupted when they are interrupted
		// 	the stale interrupted build routine (above) should never actually find anything while this is running
		qresult := db.DB.Debug().Where(&models.Image{Status: models.ImageStatusInterrupted}).Find(&images)
		if qresult.RowsAffected > 0 {
			log.WithField("numImages", qresult.RowsAffected).Info("Found image(s) with interrupted status")
		}

		for _, image := range images {
			log.WithFields(log.Fields{
				"imageID":   image.ID,
				"Account":   image.Account,
				"OrgID":     image.OrgID,
				"RequestID": image.RequestID,
			}).Info("Processing interrupted image")

			/* we have a choice here...
			1. Send an event and a consumer on Edge API calls the resume.
			2. Send an API call to Edge API to call the resume.

			Currently using the API call.
			*/

			// send an API request
			// form the internal API call from env vars and add the original requestid
			url := fmt.Sprintf("http://%s:%d/api/edge/v1/images/%d/resume", cfg.EdgeAPIServiceHost, cfg.EdgeAPIServicePort, image.ID)
			log.WithField("apiURL", url).Debug("Created the api url string")
			req, _ := http.NewRequest("POST", url, nil)
			req.Header.Add("Content-Type", "application/json")

			// recreate a stripped down identity header
			strippedIdentity := fmt.Sprintf(`{ "identity": {"account_number": "%s", "org_id": "%s", "type": "User", "internal": {"org_id": "%s"} } }`, image.Account, image.OrgID, image.OrgID)
			log.WithField("identity_text", strippedIdentity).Debug("Creating a new stripped identity")
			base64Identity := base64.StdEncoding.EncodeToString([]byte(strippedIdentity))
			log.WithField("identity_base64", base64Identity).Debug("Using a base64encoded stripped identity")
			req.Header.Add("x-rh-identity", base64Identity)

			// create a client and send a request against the Edge API
			// ignore env proxy settings to talk directly to adjacent API
			var defaultTransport http.RoundTripper = &http.Transport{Proxy: nil}
			client := &http.Client{Transport: defaultTransport}
			res, err := client.Do(req)
			if err != nil {
				var code int
				if res != nil {
					code = res.StatusCode
				}
				log.WithFields(log.Fields{
					"statusCode": code,
					"error":      err,
				}).Error("Edge API resume request error")
			} else {
				respBody, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Error("Error reading body of uninterrupted build resume response")
				}
				log.WithFields(log.Fields{
					"statusCode":   res.StatusCode,
					"responseBody": string(respBody),
					"error":        err,
				}).Debug("Edge API resume response")
				err = res.Body.Close()
				if err != nil {
					log.Error("Error closing body")
				}
			}

			// send an event on image-build topic
			// currently using the API call for the resume on the edge-api side
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
}
