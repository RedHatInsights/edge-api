package updates

import (
	"encoding/json"
	"fmt"
	"net/http"

	"context"
	"time"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	apierrors "github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// MakeRouter adds support for operations on update
func MakeRouter(sub chi.Router) {
	sub.Use(UpdateCtx)
	sub.With(common.Paginate).Get("/", GetUpdates)
	sub.Post("/", AddUpdate)
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateCtx)
		r.Get("/", GetByID)
		r.Put("/", UpdatesUpdate)
	})
}

func GetUpdates(w http.ResponseWriter, r *http.Request) {
	var updates []models.UpdateTransaction
	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// FIXME - need to sort out how to get this query to be against commit.account
	result := db.DB.Preload("DispatchRecords").Where("update_transactions.account = ?", account).Joins("Commit").Joins("Repo").Joins("Devices").Find(&updates)
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
		inventory, err = ReturnDevicesByID(w, r)
		if err != nil {
			err := apierrors.NewInternalServerError()
			err.Title = fmt.Sprintf("No devices found for UUID %s", updateJSON.DeviceUUID)
			w.WriteHeader(err.Status)
			return &models.UpdateTransaction{}, err
		}
	}

	log.Debugf("updateFromHTTP::inventory: %#v", inventory)

	// Create the models.UpdateTransaction
	update := models.UpdateTransaction{}

	// Get the models.Commit from the Commit ID passed in via JSON
	update.Commit, err = common.GetCommitByID(updateJSON.CommitID)
	log.Debugf("updateFromHTTP::update.Commit: %#v", update.Commit)
	if err != nil {
		err := apierrors.NewInternalServerError()
		err.Title = fmt.Sprintf("No commit found for CommitID %d", updateJSON.CommitID)
		w.WriteHeader(err.Status)
		return &models.UpdateTransaction{}, err
	}

	//  Check for the existence of a Repo that already has this commit and don't duplicate
	var repo *models.Repo
	repo, err = common.GetRepoByCommitID(update.Commit.ID)
	if err == nil {
		update.Repo = repo
	} else {
		if !(err.Error() == "record not found") {
			log.Errorf("updateFromHTTP::GetRepoByCommitID::repo: %#v, %#v", repo, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return &models.UpdateTransaction{}, err
		} else {
			log.Infof("Old Repo not found in database for CommitID, creating new one: %d", update.Commit.ID)
			repo := new(models.Repo)
			repo.Commit = update.Commit
			db.DB.Create(&repo)
			update.Repo = repo

		}
	}

	inventoryHosts := update.InventoryHosts
	oldCommits := update.OldCommits
	// - populate the update.InventoryHosts []Device data
	fmt.Printf("Devices in this tag %v", inventory.Result)
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
				updateDevice = new(models.Device)
				updateDevice.UUID = device.ID
				db.DB.Create(&updateDevice)
			}
		}
		updateDevice.DesiredHash = update.Commit.OSTreeCommit
		log.Debugf("updateFromHTTP::updateDevice: %#v", updateDevice)
		inventoryHosts = append(inventoryHosts, *updateDevice)
		log.Debugf("updateFromHTTP::inventoryHosts: %#v", inventoryHosts)
		update.InventoryHosts = inventoryHosts
		for _, ostreeDeployment := range device.Ostree.RpmOstreeDeployments {
			if ostreeDeployment.Booted {
				log.Debugf("updateFromHTTP::ostreeDeployment.Booted: %#v", ostreeDeployment)
				var oldCommit models.Commit
				result := db.DB.Where("os_tree_commit = ?", ostreeDeployment.Checksum).First(&oldCommit)
				log.Debugf("updateFromHTTP::result: %#v", result)
				if result.Error != nil {
					if !(result.Error.Error() == "record not found") {
						log.Errorf("updateFromHTTP::result.Error: %#v", result.Error)
						http.Error(w, result.Error.Error(), http.StatusBadRequest)
						return &models.UpdateTransaction{}, err
					} else {
						log.Infof("Old Commit not found in database: %s", ostreeDeployment.Checksum)
					}
				}
				oldCommits = append(oldCommits, oldCommit)
				update.OldCommits = oldCommits
			}
		}

	}

	log.Debugf("updateFromHTTP::update: %#v", update)
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
			"InventoryHosts": update.InventoryHosts,
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
	if update := getUpdate(w, r); update != nil {
		json.NewEncoder(w).Encode(update)
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
