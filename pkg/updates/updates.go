package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"context"
	"time"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/playbooks"

	apierrors "github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// MakeRouter adds support for operations on update
func MakeRouter(sub chi.Router) {
	sub.Use(UpdateCtx)
	sub.Get("/device/{DeviceUUID}", GetDeviceStatus)
	sub.With(common.Paginate).Get("/", GetUpdates)
	sub.Post("/", AddUpdate)
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateCtx)
		r.Get("/", GetByID)
		r.Put("/", UpdatesUpdate)
	})
}

// GetDeviceStatus returns the device with the given UUID that is associate to the account.
// This is being used for the inventory table to determine whether the current device image
// is the latest or older version.
func GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	// var devices []models.Device
	var results []models.Device
	//var results []models.UpdateTransaction
	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uuid := chi.URLParam(r, "DeviceUUID")
	result := db.DB.
		Select("desired_hash, connection_state, uuid").
		Table("devices").
		Joins(
			`JOIN updatetransaction_devices ON
			(updatetransaction_devices.device_id = devices.id AND devices.uuid = ?)`,
			uuid,
		).
		Joins(
			`JOIN update_transactions ON
			(
				update_transactions.id = updatetransaction_devices.update_transaction_id AND
				update_transactions.account = ?
			)`,
			account,
		).Find(&results)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(&results)
}

func GetUpdates(w http.ResponseWriter, r *http.Request) {
	var updates []models.UpdateTransaction
	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// FIXME - need to sort out how to get this query to be against commit.account
	result := db.DB.Preload("DispatchRecords").Preload("Devices").Where("update_transactions.account = ?", account).Joins("Commit").Joins("Repo").Find(&updates)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(&updates)
}

func isUUID(param string) bool {
	_, err := uuid.Parse(param)
	return err == nil

}

type UpdatePostJSON struct {
	CommitID   uint   `json:"CommitID"`
	Tag        string `json:"Tag"`
	DeviceUUID string `json:"DeviceUUID"`
}

func updateFromHTTP(w http.ResponseWriter, r *http.Request) (*models.UpdateTransaction, error) {
	var updateJSON UpdatePostJSON
	err := json.NewDecoder(r.Body).Decode(&updateJSON)
	log.Debugf("updateFromHTTP::updateJSON: %#v", updateJSON)

	if updateJSON.CommitID == 0 {
		err := apierrors.NewInternalServerError()
		err.Title = fmt.Sprint("Must provide a CommitID")
		w.WriteHeader(err.Status)
		return nil, err
	}
	if (updateJSON.Tag == "") && (updateJSON.DeviceUUID == "") {
		err := apierrors.NewInternalServerError()
		err.Title = fmt.Sprint("At least one of Tag or DeviceUUID required.")
		w.WriteHeader(err.Status)
		return nil, err
	}

	var inventory Inventory
	if updateJSON.Tag != "" {
		inventory, err = ReturnDevicesByTag(w, r)
		if err != nil {
			err := apierrors.NewInternalServerError()
			err.Title = fmt.Sprintf("No devices in this tag %s", updateJSON.Tag)
			w.WriteHeader(err.Status)
			return &models.UpdateTransaction{}, err
		}
	}
	if updateJSON.DeviceUUID != "" {
		headers := common.GetOutgoingHeaders(r)
		inventory, err = ReturnDevicesByID(updateJSON.DeviceUUID, headers)
		if err != nil {
			err := apierrors.NewInternalServerError()
			err.Title = fmt.Sprintf("No devices found for UUID %s", updateJSON.DeviceUUID)
			w.WriteHeader(err.Status)
			return &models.UpdateTransaction{}, err
		}
	}

	log.Infof("updateFromHTTP::inventory: %#v", inventory)

	account, err := common.GetAccount(r)
	if err != nil {
		err := apierrors.NewInternalServerError()
		err.Title = fmt.Sprintf("No account found")
		w.WriteHeader(err.Status)
		return nil, err
	}

	// Create the models.UpdateTransaction
	update := models.UpdateTransaction{
		Account:  account,
		CommitID: updateJSON.CommitID,
		Tag:      updateJSON.Tag,
	}

	// Get the models.Commit from the Commit ID passed in via JSON
	update.Commit, err = common.GetCommitByID(updateJSON.CommitID)
	log.Infof("updateFromHTTP::update.Commit: %#v", update.Commit)
	update.DispatchRecords = []models.DispatchRecord{}
	if err != nil {
		err := apierrors.NewInternalServerError()
		err.Title = fmt.Sprintf("No commit found for CommitID %d", updateJSON.CommitID)
		w.WriteHeader(err.Status)
		return &models.UpdateTransaction{}, err
	}

	//  Check for the existence of a Repo that already has this commit and don't duplicate
	var repo *models.Repo
	repo, err = common.GetRepoByCommitID(update.CommitID)
	if err == nil {
		update.Repo = repo
	} else {
		if !(err.Error() == "record not found") {
			log.Errorf("updateFromHTTP::GetRepoByCommitID::repo: %#v, %#v", repo, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return &models.UpdateTransaction{}, err
		} else {
			log.Infof("Old Repo not found in database for CommitID, creating new one: %d", update.CommitID)
			repo := &models.Repo{
				Commit: update.Commit,
				Status: models.RepoStatusBuilding,
			}
			db.DB.Create(&repo)
			update.Repo = repo
		}
	}
	log.Infof("Getting repo info: repo %s, %d", repo.URL, repo.ID)

	var remoteInfo playbooks.TemplateRemoteInfo
	remoteInfo.RemoteURL = update.Repo.URL
	remoteInfo.RemoteName = "main-test" //update.Repo.Commit.Name
	remoteInfo.ContentURL = update.Repo.URL
	remoteInfo.GpgVerify = "true"
	remoteInfo.UpdateTransaction = int(update.ID)

	playbooksURL, err := playbooks.WriteTemplate(remoteInfo, account)
	log.Infof("playbooks:WriteTemplate: %#v", playbooksURL)
	if err != nil {
		log.Errorf("Error::playbooks:WriteTemplate: %#v", err)
		err := apierrors.NewInternalServerError()
		err.Title = "Error during playbook creation"
		w.WriteHeader(err.Status)
	}

	devices := update.Devices
	oldCommits := update.OldCommits
	dispatchRecords := update.DispatchRecords
	// - populate the update.Devices []Device data
	log.Infof("Devices in this tag %v", inventory.Result)
	for _, device := range inventory.Result {
		//  Check for the existence of a Repo that already has this commit and don't duplicate
		var updateDevice *models.Device
		updateDevice, err = common.GetDeviceByUUID(device.ID)
		if err != nil {
			if !(err.Error() == "record not found") {
				log.Errorf("updateFromHTTP::GetDeviceByUUID::updateDevice: %#v, %#v", repo, err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return &models.UpdateTransaction{}, err
			} else {
				log.Infof("Existing Device not found in database, creating new one: %s", device.ID)
				updateDevice = &models.Device{
					UUID:        device.ID,
					RHCClientID: device.Ostree.RHCClientID,
				}
				db.DB.Create(&updateDevice)
			}
		}
		updateDevice.DesiredHash = update.Commit.OSTreeCommit
		log.Debugf("updateFromHTTP::updateDevice: %#v", updateDevice)
		devices = append(devices, *updateDevice)
		log.Debugf("updateFromHTTP::devices: %#v", devices)
		update.Devices = devices
		log.Debugf("updateFromHTTP::update.Devices: %#v", devices)

		// - Call playbook dispatcher
		var payloadDispatcher playbooks.DispatcherPayload
		payloadDispatcher.Recipient = updateDevice.RHCClientID
		payloadDispatcher.PlaybookURL = playbooksURL
		payloadDispatcher.Account = update.Account

		log.Infof("Call Execute Dispatcher")
		log.Infof("payload Dispatcher::  %#v", payloadDispatcher)
		exc, err := playbooks.ExecuteDispatcher(payloadDispatcher)

		if err != nil {
			log.Errorf("Error on playbook-dispatcher-executuin: %#v ", err.Error())
			return nil, err
		}
		for _, excPlaybook := range exc {
			if excPlaybook.StatusCode == http.StatusCreated {
				updateDevice.ConnectionState = true
				dispatchRecord := &models.DispatchRecord{
					Device:               updateDevice,
					PlaybookURL:          playbooksURL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				updateDevice.ConnectionState = false
				dispatchRecord := &models.DispatchRecord{
					Device:      updateDevice,
					PlaybookURL: playbooksURL,
					Status:      models.DispatchRecordStatusError,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			}

		}
		update.DispatchRecords = dispatchRecords
		log.Infof("updateFromHTTP::dispatchRecords: %#v", dispatchRecords)
		log.Infof("updateFromHTTP::update.devices: %#v", devices)
		// - end playbook dispatcher

		for _, ostreeDeployment := range device.Ostree.RpmOstreeDeployments {
			if ostreeDeployment.Booted {
				log.Infof("updateFromHTTP::ostreeDeployment.Booted: %#v", ostreeDeployment)
				var oldCommit models.Commit
				result := db.DB.Where("os_tree_commit = ?", ostreeDeployment.Checksum).First(&oldCommit)
				log.Infof("updateFromHTTP::result: %#v", result)
				if result.Error != nil {
					if result.Error.Error() != "record not found" {
						log.Errorf("updateFromHTTP::result.Error: %#v", result.Error)
						http.Error(w, result.Error.Error(), http.StatusBadRequest)
						return &models.UpdateTransaction{}, err
					}
				}
				if result.RowsAffected == 0 {
					log.Infof("Old Commit not found in database: %s", ostreeDeployment.Checksum)
				} else {
					oldCommits = append(oldCommits, oldCommit)
				}
			}
		}
	}
	update.OldCommits = oldCommits

	log.Infof("updateFromHTTP::update: %#v", update)
	return &update, err
}

type key int

const UpdateContextKey key = 0

// Implement Context interface so we can shuttle around multiple values
type UpdateContext struct {
	DeviceUUID string
	Tag        string
}

// UpdateCtx is a handler for Update requests
func UpdateCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uCtx UpdateContext
		uCtx.DeviceUUID = chi.URLParam(r, "DeviceUUID")

		uCtx.Tag = chi.URLParam(r, "Tag")
		log.Debugf("UpdateCtx::uCtx: %#v", uCtx)
		ctx := context.WithValue(r.Context(), UpdateContextKey, &uCtx)
		log.Debugf("UpdateCtx::ctx: %#v", ctx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AddUpdate adds an object to the database for an account
func AddUpdate(w http.ResponseWriter, r *http.Request) {

	update, err := updateFromHTTP(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Debugf("AddUpdate::update: %#v", update)

	update.Account, err = common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check to make sure we're not duplicating the job
	// FIXME - this didn't work and I don't have time to debug right now
	// FIXME - handle UpdateTransaction Commit vs UpdateCommitID
	/*
		var dupeRecord models.UpdateTransaction
		queryDuplicate := map[string]interface{}{
			"Account":        update.Account,
			"Devices": update.Devices,
			"OldCommitIDs":   update.OldCommitIDs,
		}
		result := db.DB.Where(queryDuplicate).Find(&dupeRecord)
		if result.Error == nil {
			if dupeRecord.UpdateCommitID != 0 {
				http.Error(w, "Can not submit duplicate update job", http.StatusInternalServerError)
				return
			}
		}
	*/

	// FIXME - need to remove duplicate OldCommit values from UpdateTransaction

	json.NewEncoder(w).Encode(&update)
	result := db.DB.Create(&update)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
	}

	go commits.RepoBuilderInstance.BuildUpdateRepo(update)

}

// GetByID obtains an update from the database for an account
func GetByID(w http.ResponseWriter, r *http.Request) {
	var update models.UpdateTransaction

	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if updateID := chi.URLParam(r, "updateID"); updateID != "" {
		id, err := strconv.Atoi(updateID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result := db.DB.Preload("DispatchRecords").Preload("Devices").Where("update_transactions.account = ?", account).Joins("Commit").Joins("Repo").Find(&update, id)
		if result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(update)
	} else {
		json.NewEncoder(w).Encode(&models.UpdateTransaction{})
	}
}

// UpdatesUpdate a update object in the database for an an account
func UpdatesUpdate(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		return
	}

	incoming, err := updateFromHTTP(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	incoming.ID = update.ID
	incoming.CreatedAt = now
	incoming.UpdatedAt = now
	db.DB.Save(&incoming)

	json.NewEncoder(w).Encode(incoming)
}

func getUpdate(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction {
	ctx := r.Context()
	update, ok := ctx.Value(UpdateContextKey).(*models.UpdateTransaction)
	if !ok {
		http.Error(w, "must pass id", http.StatusBadRequest)
		return nil
	}
	return update
}
