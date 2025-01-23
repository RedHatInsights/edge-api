// FIXME: golangci-lint
// nolint:errcheck,govet,revive,unused
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/redhatinsights/edge-api/config"
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
	qresult := db.DB.Where(query).Find(&images)
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
	tx := db.DB.Model(&models.Image{}).Where("ID = ?", id).Update("Status", status)
	if tx.Error != nil {
		log.WithField("error", tx.Error.Error()).Error("Error updating image status")
		return tx.Error
	}

	log.WithField("imageID", id).Debug("Image updated with " + fmt.Sprint(status) + " status")

	return nil
}

func main() {
	ctx := context.Background()
	config.Init()
	cfg := config.Get()
	err := logger.InitializeLogging(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer logger.Flush()

	var images []models.Image
	// IBevent represents the struct of the value in a Kafka message
	// TODO: add the original requestid
	type IBevent struct {
		ImageID uint `json:"image_id"`
	}

	config.LogConfigAtStartup(cfg)
	db.InitDB()

	log.Info("Entering the infinite loop...")
	for {
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
		qresult := db.DB.Where(&models.Image{Status: models.ImageStatusInterrupted}).Find(&images)
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
				}).Error("Edge API resume request error")
			} else {
				respBody, err := io.ReadAll(res.Body)
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

			// removed the old event producer and will replace with new Producer code when ready to move away from API call
			// it goes here
		}
	}
}
