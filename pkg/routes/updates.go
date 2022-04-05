package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

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
		r.Get("/notify", SendNotificationForDevice) //TMP ROUTE TO SEND THE NOTIFICATION
	})
	// TODO: This is for backwards compatibility with the previous route
	// Once the frontend starts querying the device
	sub.Route("/device/", MakeDevicesRouter)
}

type updateContextKey int

// UpdateContextKey is the key to Update Context handler
const UpdateContextKey updateContextKey = iota

// UpdateCtx is a handler for Update requests
func UpdateCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		services := dependencies.ServicesFromContext(r.Context())
		var update models.UpdateTransaction
		account, err := common.GetAccount(r)
		if err != nil {
			services.Log.WithFields(log.Fields{
				"error":   err.Error(),
				"account": account,
			}).Error("Error retrieving account")
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
			return
		}
		updateID := chi.URLParam(r, "updateID")
		services.Log = services.Log.WithField("updateID", updateID)
		if updateID == "" {
			err := errors.NewBadRequest("UpdateTransactionID can't be empty")
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
			return
		}
		id, err := strconv.Atoi(updateID)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
			return
		}
		result := db.DB.Preload("DispatchRecords").Preload("Devices").Where("update_transactions.account = ?", account).Joins("Commit").Joins("Repo").Find(&update, id)
		if result.Error != nil {
			services.Log.WithFields(log.Fields{
				"error": result.Error.Error(),
			}).Error("Error retrieving update")
			err := errors.NewInternalServerError()
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
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
		// Error set by UpdateCtx already
		return
	}
	services := dependencies.ServicesFromContext(r.Context())
	playbook, err := services.UpdateService.GetUpdatePlaybook(update)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error getting update playbook")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	defer playbook.Close()
	_, err = io.Copy(w, playbook)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error reading the update playbook")
		err := errors.NewInternalServerError()
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
}

// GetUpdates returns the updates for the device
func GetUpdates(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	var updates []models.UpdateTransaction
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithFields(log.Fields{
			"error":   err.Error(),
			"account": account,
		}).Error("Error retrieving account")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}
	// FIXME - need to sort out how to get this query to be against commit.account
	result := db.DB.Preload("DispatchRecords").Preload("Devices").Where("update_transactions.account = ?", account).Joins("Commit").Joins("Repo").Find(&updates)
	if result.Error != nil {
		services.Log.WithFields(log.Fields{
			"error": result.Error.Error(),
		}).Error("Error retrieving updates")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return
	}

	if err := json.NewEncoder(w).Encode(&updates); err != nil {
		services.Log.WithField("error", updates).Error("Error while trying to encode")
	}
}

//DevicesUpdate contains the update structure for the device
type DevicesUpdate struct {
	CommitID    uint     `json:"CommitID"`
	DevicesUUID []string `json:"DeviceUUID"`
	// TODO: Implement updates by tag
	// Tag        string `json:"Tag"`
}

func updateFromHTTP(w http.ResponseWriter, r *http.Request) (*[]models.UpdateTransaction, error) {
	services := dependencies.ServicesFromContext(r.Context())
	services.Log.Info("Update is being created")

	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithFields(log.Fields{
			"error":   err.Error(),
			"account": account,
		}).Error("Error retrieving account")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		if err := json.NewEncoder(w).Encode(&err); err != nil {
			services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
		}
		return nil, err
	}

	var devicesUpdate DevicesUpdate
	err = json.NewDecoder(r.Body).Decode(&devicesUpdate)
	if err != nil {
		err := errors.NewBadRequest("Invalid JSON")
		w.WriteHeader(err.GetStatus())
		return nil, err
	}
	services.Log.WithField("updateJSON", devicesUpdate).Debug("Update JSON received")

	if devicesUpdate.CommitID == 0 {
		err := errors.NewBadRequest("Must provide a CommitID")
		w.WriteHeader(err.GetStatus())
		return nil, err
	}
	// TODO: Implement update by tag - Add validation per tag
	if devicesUpdate.DevicesUUID == nil {
		err := errors.NewBadRequest("DeviceUUID required.")
		w.WriteHeader(err.GetStatus())
		return nil, err
	}
	client := inventory.InitClient(r.Context(), log.NewEntry(log.StandardLogger()))
	var inv inventory.Response
	var ii []inventory.Response
	if len(devicesUpdate.DevicesUUID) > 0 {
		for _, UUID := range devicesUpdate.DevicesUUID {
			inv, err = client.ReturnDevicesByID(UUID)
			if inv.Count >= 0 {
				ii = append(ii, inv)
			}
			if err != nil {
				err := errors.NewNotFound(fmt.Sprintf("No devices found for UUID %s", UUID))
				w.WriteHeader(err.GetStatus())
				return nil, err
			}
		}
	}

	services.Log.WithField("inventoryDevice", inv).Debug("Device retrieved from inventory")
	var updates []models.UpdateTransaction
	for _, inventory := range ii {
		// Create the models.UpdateTransaction
		update := models.UpdateTransaction{
			Account:  account,
			CommitID: devicesUpdate.CommitID,
			Status:   models.UpdateStatusCreated,
			// TODO: Implement update by tag
			// Tag:      updateJSON.Tag,
		}

		// Get the models.Commit from the Commit ID passed in via JSON
		update.Commit, err = services.CommitService.GetCommitByID(devicesUpdate.CommitID)
		services.Log.WithField("commit", update.Commit).Debug("Commit retrieved from this update")

		notify, errNotify := services.UpdateService.SendDeviceNotification(&update)
		if errNotify != nil {
			services.Log.WithField("message", errNotify.Error()).Error("Error to send notification")
			services.Log.WithField("message", notify).Error("Notify Error")

		}
		update.DispatchRecords = []models.DispatchRecord{}
		if err != nil {
			services.Log.WithFields(log.Fields{
				"error":    err.Error(),
				"commitID": devicesUpdate.CommitID,
			}).Error("No commit found for Commit ID")
			err := errors.NewInternalServerError()
			err.SetTitle(fmt.Sprintf("No commit found for CommitID %d", devicesUpdate.CommitID))
			w.WriteHeader(err.GetStatus())
			return nil, err
		}

		//  Removing commit dependency to avoid overwriting the repo
		var repo *models.Repo
		services.Log.WithField("updateID", update.ID).Debug("Ceating new repo for update transaction")
		repo = &models.Repo{
			Status: models.RepoStatusBuilding,
		}
		result := db.DB.Create(&repo)
		if result.Error != nil {
			services.Log.WithField("error", result.Error.Error()).Debug("Result error")
			err := errors.NewBadRequest(result.Error.Error())
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", result.Error.Error()).Error("Error while trying to encode")
			}
		}
		update.Repo = repo
		services.Log.WithFields(log.Fields{
			"repoURL": repo.URL,
			"repoID":  repo.ID,
		}).Debug("Getting repo info")

		devices := update.Devices
		oldCommits := update.OldCommits

		for _, device := range inventory.Result {
			fmt.Printf("DeviceUUID: %v\n", device.ID)
			//  Check for the existence of a Repo that already has this commit and don't duplicate
			var updateDevice *models.Device
			updateDevice, err = services.DeviceService.GetDeviceByUUID(device.ID)
			if err != nil {
				if !(err.Error() == "Device was not found") {
					services.Log.WithField("error", err.Error()).Error("Device was not found in our database")
					err := errors.NewBadRequest(err.Error())
					w.WriteHeader(err.GetStatus())
					if err := json.NewEncoder(w).Encode(&err); err != nil {
						services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
					}
					return nil, err
				}
				services.Log.WithFields(log.Fields{
					"error":      err.Error(),
					"deviceUUID": device.ID,
				}).Info("Creating a new device on the database")
				updateDevice = &models.Device{
					UUID:    device.ID,
					Account: account,
				}
				if result := db.DB.Create(&updateDevice); result.Error != nil {
					return nil, result.Error
				}
			}
			updateDevice.RHCClientID = device.Ostree.RHCClientID
			updateDevice.AvailableHash = update.Commit.OSTreeCommit
			// update the device account if undefined
			if updateDevice.Account == "" {
				updateDevice.Account = account
			}
			result := db.DB.Save(&updateDevice)
			if result.Error != nil {
				return nil, result.Error
			}

			services.Log.WithFields(log.Fields{
				"updateDevice": updateDevice,
			}).Debug("Saved updated device")

			devices = append(devices, *updateDevice)
			update.Devices = devices

			for _, deployment := range device.Ostree.RpmOstreeDeployments {
				services.Log.WithFields(log.Fields{
					"ostreeDeployment": deployment,
				}).Debug("Got ostree deployment for device")
				if deployment.Booted {
					services.Log.WithFields(log.Fields{
						"booted": deployment.Booted,
					}).Debug("device has been booted")
					var oldCommit models.Commit
					result := db.DB.Where("os_tree_commit = ?", deployment.Checksum).First(&oldCommit)
					if result.Error != nil {
						if result.Error.Error() != "record not found" {
							services.Log.WithField("error", err.Error()).Error("Error returning old commit for this ostree checksum")
							err := errors.NewBadRequest(err.Error())
							w.WriteHeader(err.GetStatus())
							if err := json.NewEncoder(w).Encode(&err); err != nil {
								services.Log.WithField("error", err.Error()).Error("Error encoding error")
							}
							return nil, err
						}
					}
					if result.RowsAffected == 0 {
						services.Log.Debug("No old commits found")
					} else {
						oldCommits = append(oldCommits, oldCommit)
					}
				}
			}

			update.OldCommits = oldCommits
			if err := db.DB.Save(&update).Error; err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				if err := json.NewEncoder(w).Encode(&err); err != nil {
					services.Log.WithField("error", err.Error()).Error("Error encoding error")
				}
				return nil, err
			}
		}
		updates = append(updates, update)
		services.Log.WithField("updateID", update.ID).Info("Update has been created")

	}
	return &updates, nil
}

// AddUpdate updates a device
func AddUpdate(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	services.Log.Info("Starting update")
	updates, err := updateFromHTTP(w, r)
	if err != nil {
		services.Log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error building update from request")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		return
	}
	for _, update := range *updates {
		update.Account, err = common.GetAccount(r)
		if err != nil {
			services.Log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Error retrieving account")
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.GetStatus())
			return
		}
		result := db.DB.Save(&update)
		if result.Error != nil {
			services.Log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("Error saving update")
			err := errors.NewInternalServerError()
			w.WriteHeader(err.GetStatus())
			return
		}
		services.Log.WithField("updateID", update.ID).Info("Starting asynchronous update process")
		go services.UpdateService.CreateUpdate(update.ID)
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updates); err != nil {
		services.Log.WithField("error", updates).Error("Error while trying to encode")
	}

}

// GetUpdateByID obtains an update from the database for an account
func GetUpdateByID(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		// Error set by UpdateCtx already
		return
	}
	if err := json.NewEncoder(w).Encode(update); err != nil {
		services := dependencies.ServicesFromContext(r.Context())
		services.Log.WithField("error", update).Error("Error while trying to encode")
	}
}

func getUpdate(w http.ResponseWriter, r *http.Request) *models.UpdateTransaction {
	ctx := r.Context()
	update, ok := ctx.Value(UpdateContextKey).(*models.UpdateTransaction)
	if !ok {
		// Error set by UpdateCtx already
		return nil
	}
	return update
}

//SendNotificationForDevice TMP route to validate
func SendNotificationForDevice(w http.ResponseWriter, r *http.Request) {
	if update := getUpdate(w, r); update != nil {
		services := dependencies.ServicesFromContext(r.Context())
		notify, err := services.UpdateService.SendDeviceNotification(update)
		if err != nil {
			services.Log.WithField("error", err.Error()).Error("Failed to retry to send notification")
			err := errors.NewInternalServerError()
			err.SetTitle("Failed creating image")
			w.WriteHeader(err.GetStatus())
			if err := json.NewEncoder(w).Encode(&err); err != nil {
				services.Log.WithField("error", err.Error()).Error("Error while trying to encode")
			}
			return
		}
		services.Log.WithField("StatusOK", http.StatusOK).Info("Writting Header")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(notify); err != nil {
			services.Log.WithField("error", notify).Error("Error while trying to encode")
		}
	}
}
