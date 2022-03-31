package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	//"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

// NOTE: this is currently designed for a single ibvents replica

func getStaleBuilds(status string, age int) []models.Image {
	var images []models.Image

	// check the database for image builds in INTERRUPTED status
	//qresult := db.DB.Debug().Where(&models.Image{Status: models.ImageStatusInterrupted}).Find(&images)
	//qresult := db.DB.Debug().Raw("SELECT id, status, updated_at FROM images WHERE status = ? AND updated_at < NOW() - INTERVAL '? hours'", status, age).Scan(&images)
	//tresult := db.DB.Debug().Raw("SELECT id, status, updated_at FROM images WHERE status = ? AND updated_at < NOW() - INTERVAL '? hours'", status, age).Scan(&images)

	// temporary query to see this thing work
	tresult := db.DB.Debug().Where("status = '?' AND updated_at < NOW() - INTERVAL '? hours'", status, age).Find(&images)
	if tresult.Error != nil {
		log.WithField("error", tresult.Error.Error()).Error("tresult query failed")
	} else {
		log.WithFields(log.Fields{
			"numImages": tresult.RowsAffected,
			"status":    status,
		}).Info("Found testing image(s) with interval")

		for _, staleImage := range images {
			log.WithFields(log.Fields{
				"UpdatedAt": staleImage.UpdatedAt,
				"ID":        staleImage.ID,
				"Status":    staleImage.Status,
			}).Info("Logging test interrupted image")
		}
	}

	qresult := db.DB.Debug().Raw("SELECT id, status, updated_at FROM images WHERE status = '?'", status).Scan(&images)
	log.WithFields(log.Fields{
		"numImages": qresult.RowsAffected,
		"status":    status,
	}).Info("Found stale image(s)")

	return images
}

func setImageStatus(id uint, status string) error {
	/*tx := db.DB.Debug().Model(&models.Image{}).Where("ID = ?", id).Update("Status", status)
	log.WithField("imageID", id).Debug("Image updated with " + fmt.Sprint(status) + " status")
	if tx.Error != nil {
		log.WithField("error", tx.Error.Error()).Error("Error updating image status")
		return tx.Error
	}
	*/
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
	// FIXME: EdgeAPIURL is the external URL, not internal svc
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
		"EdgeAPIURL":               cfg.EdgeAPIBaseURL,
	}).Info("Configuration Values:")
	db.InitDB()

	log.Info("Entering the infinite loop...")
	for {
		log.Debug("Sleeping...")
		time.Sleep(5 * time.Minute)
		// TODO: work out programatic method to avoid resuming a build until app is up or on way up

		// handle stale interrupted builds not complete after x hours
		staleInterruptedImages := getStaleBuilds(models.ImageStatusInterrupted, 48)
		for _, staleImage := range staleInterruptedImages {
			log.WithFields(log.Fields{
				"UpdatedAt": staleImage.UpdatedAt,
				"ID":        staleImage.ID,
				"Status":    staleImage.Status,
			}).Info("Processing stale interrupted image")

			// FIXME: holding off on db update until we see the query work in stage
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

			// FIXME: holding off on db update until we see the query work in stage
			statusUpdateError := setImageStatus(staleImage.ID, models.ImageStatusError)
			if statusUpdateError != nil {
				log.Error("Failed to update stale building image build status")
			}
		}

		// handle image builds in INTERRUPTED status
		qresult := db.DB.Debug().Where(&models.Image{Status: models.ImageStatusInterrupted}).Find(&images)
		log.WithField("numImages", qresult.RowsAffected).Info("Found image(s) with interrupted status")

		for _, image := range images {
			log.WithField("imageID", image.ID).Info("Processing interrupted image")

			/* we have a choice here...
			1. Send an event and a consumer on Edge API calls the resume.
			2. Send an API call to Edge API to call the resume.

			Currently...
			1. Testing a Kafka event.
			2. Will implement a call to the API /retry
			3. Will create an API endpoint specifically for resume()
				so it can pick up where it left off
			*/

			/* temp disable until auth can be resolved
			// send an API request
			url := fmt.Sprintf("%s/api/edge/v1/images/%d/retry", cfg.EdgeAPIBaseURL, image.ID)
			req, _ := http.NewRequest("POST", url, nil)
			req.Header.Add("Content-Type", "application/json")

			client := &http.Client{}
			res, err := client.Do(req)
			if err != nil {
				var code int
				if res != nil {
					code = res.StatusCode
				}
				log.WithFields(log.Fields{
					"statusCode": code,
					"error":      err,
				}).Error("Edge API retry request error")
			}
			respBody, err := ioutil.ReadAll(res.Body)
			log.WithFields(log.Fields{
				"statusCode":   res.StatusCode,
				"responseBody": string(respBody),
				"error":        err,
			}).Debug("Edge API retry response")
			if err != nil {
				log.Error("Error reading body of uninterrupted build resume response")
			}
			res.Body.Close()
			*/

			// send an event on image-build topic
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
