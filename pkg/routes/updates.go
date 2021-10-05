package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	log "github.com/sirupsen/logrus"
)

// MakeUpdatesRouter adds support for operations on update
func MakeUpdatesRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", GetUpdates)
	sub.Post("/", AddUpdate)
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateCtx)
		r.Get("/", GetUpdateByID)
		r.Get("/update-playbook.yml", GetUpdatePlaybook)
	})
	// TODO: This is for backwards compatibility with the previous route
	// Once the frontend starts querying the device
	sub.Route("/device/", MakeDevicesRouter)
}

type updateContextKey int

const UpdateContextKey updateContextKey = iota
const DelayTimeToReboot = 10

// UpdateCtx is a handler for Update requests
func UpdateCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var update models.UpdateTransaction
		account, err := common.GetAccount(r)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		updateID := chi.URLParam(r, "updateID")
		if updateID == "" {
			err := errors.NewBadRequest("UpdateTransactionID can't be empty")
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		id, err := strconv.Atoi(updateID)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		result := db.DB.Preload("DispatchRecords").Preload("Devices").Where("update_transactions.account = ?", account).Joins("Commit").Joins("Repo").Find(&update, id)
		if result.Error != nil {
			err := errors.NewInternalServerError()
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		ctx := context.WithValue(r.Context(), UpdateContextKey, &update)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUpdatePlaybook returns the playbook for a update transaction
func GetUpdatePlaybook(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		return
	}
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	playbook, err := services.UpdateService.GetUpdatePlaybook(update)
	if err != nil {
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	defer playbook.Close()
	_, err = io.Copy(w, playbook)
	if err != nil {
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
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

type UpdatePostJSON struct {
	CommitID   uint   `json:"CommitID"`
	DeviceUUID string `json:"DeviceUUID"`
	// TODO: Implement updates by tag
	// Tag        string `json:"Tag"`
}

func updateFromHTTP(w http.ResponseWriter, r *http.Request) (*models.UpdateTransaction, error) {
	log.Infof("updateFromHTTP:: Begin")

	account, err := common.GetAccount(r)
	if err != nil {
		err := errors.NewInternalServerError()
		err.Title = "No account found"
		w.WriteHeader(err.Status)
		return nil, err
	}

	var updateJSON UpdatePostJSON
	err = json.NewDecoder(r.Body).Decode(&updateJSON)
	if err != nil {
		err := errors.NewBadRequest("Invalid JSON")
		w.WriteHeader(err.Status)
		return nil, err
	}
	log.Infof("updateFromHTTP::updateJSON: %#v", updateJSON)

	if updateJSON.CommitID == 0 {
		err := errors.NewBadRequest("Must provide a CommitID")
		w.WriteHeader(err.Status)
		return nil, err
	}
	// TODO: Implement update by tag - Add validation per tag
	if updateJSON.DeviceUUID == "" {
		err := errors.NewBadRequest("DeviceUUID required.")
		w.WriteHeader(err.Status)
		return nil, err
	}
	client := inventory.InitClient(r.Context())
	var inventory inventory.Response
	// TODO: Implement update by tag
	// if updateJSON.Tag != "" {
	// 	inventory, err = client.ReturnDevicesByTag(updateJSON.Tag)
	// 	if err != nil || inventory.Count == 0 {
	// 		err := errors.NewNotFound(fmt.Sprintf("No devices found for Tag %s", updateJSON.Tag))
	// 		w.WriteHeader(err.Status)
	// 		return nil, err
	// 	}
	// }
	if updateJSON.DeviceUUID != "" {
		inventory, err = client.ReturnDevicesByID(updateJSON.DeviceUUID)
		if err != nil || inventory.Count == 0 {
			err := errors.NewNotFound(fmt.Sprintf("No devices found for UUID %s", updateJSON.DeviceUUID))
			w.WriteHeader(err.Status)
			return nil, err
		}
	}

	log.Infof("updateFromHTTP::inventory: %#v", inventory)

	// Create the models.UpdateTransaction
	update := models.UpdateTransaction{
		Account:  account,
		CommitID: updateJSON.CommitID,
		Status:   models.UpdateStatusCreated,
		// TODO: Implement update by tag
		// Tag:      updateJSON.Tag,
	}

	// Get the models.Commit from the Commit ID passed in via JSON
	commitService := services.NewCommitService()
	update.Commit, err = commitService.GetCommitByID(updateJSON.CommitID)
	log.Infof("updateFromHTTP::update.Commit: %#v", update.Commit)
	update.DispatchRecords = []models.DispatchRecord{}
	if err != nil {
		err := errors.NewInternalServerError()
		err.Title = fmt.Sprintf("No commit found for CommitID %d", updateJSON.CommitID)
		w.WriteHeader(err.Status)
		return &models.UpdateTransaction{}, err
	}

	//  Removing commit dependency to avoid overwriting the repo
	var repo *models.Repo
	log.Infof("creating new repo for update transaction: %d", update.ID)
	repo = &models.Repo{
		Status: models.RepoStatusBuilding,
	}
	db.DB.Create(&repo)
	update.Repo = repo
	log.Infof("Getting repo info: repo %s, %d", repo.URL, repo.ID)

	devices := update.Devices
	oldCommits := update.OldCommits

	// - populate the update.Devices []Device data
	log.Infof("Devices in this tag %v", inventory.Result)
	for _, device := range inventory.Result {
		//  Check for the existence of a Repo that already has this commit and don't duplicate
		var updateDevice *models.Device
		deviceService := services.NewDeviceService(r.Context())
		updateDevice, err = deviceService.GetDeviceByUUID(device.ID)
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
		log.Infof("updateFromHTTP::updateDevice: %#v", updateDevice)
		devices = append(devices, *updateDevice)
		log.Infof("updateFromHTTP::devices: %#v", devices)
		update.Devices = devices
		log.Infof("updateFromHTTP::update.Devices: %#v", devices)

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
	if err := db.DB.Save(&update).Error; err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}
	log.Infof("updateFromHTTP::update: %#v", update)
	log.Infof("updateFromHTTP:: END")
	return &update, nil
}

// AddUpdate adds an object to the database for an account
func AddUpdate(w http.ResponseWriter, r *http.Request) {
	log.Infof("AddUpdate::update:: Begin")
	update, err := updateFromHTTP(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Infof("AddUpdate::update: %#v", update)

	update.Account, err = common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := db.DB.Save(&update)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
	}
	service := services.NewUpdateService(r.Context())
	log.Infof("AddUpdate:: call::	service.CreateUpdate :: %d", update.ID)
	go service.CreateUpdate(update)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(update)

	timer := time.AfterFunc(time.Minute*DelayTimeToReboot, func() {
		services.NewUpdateService(r.Context()).RebootDevice(update)
	})
	defer timer.Stop()

}

// GetUpdateByID obtains an update from the database for an account
func GetUpdateByID(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		return
	}
	json.NewEncoder(w).Encode(update)
}

func getUpdate(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction {
	ctx := r.Context()
	update, ok := ctx.Value(UpdateContextKey).(*models.UpdateTransaction)
	if !ok {
		return nil
	}
	return update
}
