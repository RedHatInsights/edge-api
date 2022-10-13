// FIXME: golangci-lint
// nolint:errcheck,gocritic,govet,revive
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"gorm.io/gorm"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// UpdateServiceInterface defines the interface that helps
// handle the business logic of sending updates to a edge device
type UpdateServiceInterface interface {
	BuildUpdateTransactions(devicesUpdate *models.DevicesUpdate, orgID string, commit *models.Commit) (*[]models.UpdateTransaction, error)
	CreateUpdate(id uint) (*models.UpdateTransaction, error)
	CreateUpdateAsync(id uint)
	GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error)
	GetUpdateTransactionsForDevice(device *models.Device) (*[]models.UpdateTransaction, error)
	ProcessPlaybookDispatcherRunEvent(message []byte) error
	WriteTemplate(templateInfo TemplateRemoteInfo, orgID string) (string, error)
	SetUpdateStatusBasedOnDispatchRecord(dispatchRecord models.DispatchRecord) error
	SetUpdateStatus(update *models.UpdateTransaction) error
	SendDeviceNotification(update *models.UpdateTransaction) (ImageNotification, error)
	UpdateDevicesFromUpdateTransaction(update models.UpdateTransaction) error
	ValidateUpdateSelection(orgID string, imageIds []uint) (bool, error)
	ValidateUpdateDeviceGroup(orgID string, deviceGroupID uint) (bool, error)
}

// NewUpdateService gives a instance of the main implementation of a UpdateServiceInterface
func NewUpdateService(ctx context.Context, log *log.Entry) UpdateServiceInterface {
	return &UpdateService{
		Service:        Service{ctx: ctx, log: log.WithField("service", "update")},
		FilesService:   NewFilesService(log),
		ImageService:   NewImageService(ctx, log),
		RepoBuilder:    NewRepoBuilder(ctx, log),
		Inventory:      inventory.InitClient(ctx, log),
		PlaybookClient: playbookdispatcher.InitClient(ctx, log),
		WaitForReboot:  time.Minute * 5,
	}
}

// UpdateService is the main implementation of a UpdateServiceInterface
type UpdateService struct {
	Service
	ImageService   ImageServiceInterface
	RepoBuilder    RepoBuilderInterface
	FilesService   FilesService
	DeviceService  DeviceServiceInterface
	Inventory      inventory.ClientInterface
	PlaybookClient playbookdispatcher.ClientInterface
	WaitForReboot  time.Duration
}

type playbooks struct {
	GoTemplateRemoteName string
	GoTemplateGpgVerify  string
	OstreeRemoteName     string
	OstreeGpgVerify      string
	OstreeGpgKeypath     string
	UpdateNumber         string
	RepoURL              string
	RepoContentURL       string
	RemoteOstreeUpdate   string
	OSTreeRef            string
}

// TemplateRemoteInfo the values to playbook
type TemplateRemoteInfo struct {
	RemoteName          string
	RemoteURL           string
	ContentURL          string
	GpgVerify           string
	UpdateTransactionID uint
	RemoteOstreeUpdate  string
	OSTreeRef           string
}

// PlaybookDispatcherEventPayload belongs to PlaybookDispatcherEvent
type PlaybookDispatcherEventPayload struct {
	ID            string `json:"id"`
	OrgID         string `json:"org_id"`
	Recipient     string `json:"recipient"`
	CorrelationID string `json:"correlation_id"`
	Service       string `json:"service"`
	URL           string `json:"url"`
	Labels        struct {
		ID      string `json:"id"`
		StateID string `json:"state_id"`
	} `json:"labels"`
	Status    string    `json:"status"`
	Timeout   int       `json:"timeout"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlaybookDispatcherEvent is the event that gets sent to the Kafka broker when a update finishes
type PlaybookDispatcherEvent struct {
	EventType string                         `json:"event_type"`
	Payload   PlaybookDispatcherEventPayload `json:"payload"`
}

// CreateUpdateAsync is the function that creates an update transaction asynchronously
func (s *UpdateService) CreateUpdateAsync(id uint) {
	go s.CreateUpdate(id)
}

// CreateUpdate is the function that creates an update transaction
func (s *UpdateService) CreateUpdate(id uint) (*models.UpdateTransaction, error) {
	var update *models.UpdateTransaction
	db.DB.Preload("DispatchRecords").Preload("Devices").Joins("Commit").Joins("Repo").Find(&update, id)
	update.Status = models.UpdateStatusBuilding
	db.DB.Model(&models.UpdateTransaction{}).Where("ID=?", update.ID).Update("Status", update.Status)
	WaitGroup.Add(1) // Processing one update
	defer func() {
		WaitGroup.Done() // Done with one update (successfully or not)
		s.log.Debug("Done with one update - successfully or not")
		if err := recover(); err != nil {
			s.log.WithField("error", err).Error("Error on update")
		}
	}()

	go func(update *models.UpdateTransaction) {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		sig := <-sigint
		// Reload update to get updated status
		db.DB.First(&update, update.ID)
		if update.Status == models.UpdateStatusBuilding {
			s.log.WithFields(log.Fields{
				"signal":   sig,
				"updateID": update.ID,
			}).Info("Captured signal marking update as error")
			update.Status = models.UpdateStatusError
			tx := db.DB.Save(update)
			if tx.Error != nil {
				s.log.WithField("error", tx.Error.Error()).Error("Error saving update")
			}
			WaitGroup.Done()
		}
	}(update)

	update, err := s.RepoBuilder.BuildUpdateRepo(id)
	if err != nil {
		db.DB.First(&update, id)
		update.Status = models.UpdateStatusError
		db.DB.Save(update)
		s.log.WithField("error", err.Error()).Error("Error building update repo")
		return nil, err
	}

	var remoteInfo TemplateRemoteInfo
	remoteInfo.RemoteURL = update.Repo.URL
	remoteInfo.RemoteName = "rhel-edge"
	remoteInfo.ContentURL = update.Repo.URL
	remoteInfo.UpdateTransactionID = update.ID
	remoteInfo.GpgVerify = "false"
	remoteInfo.OSTreeRef = update.Commit.OSTreeRef
	remoteInfo.RemoteOstreeUpdate = fmt.Sprint(update.ChangesRefs)

	playbookURL, err := s.WriteTemplate(remoteInfo, update.OrgID)

	if err != nil {
		update.Status = models.UpdateStatusError
		db.DB.Save(update)
		s.log.WithField("error", err.Error()).Error("Error writing playbook template")
		return nil, err
	}
	// get the content identity
	indent, err := common.GetIdentityFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting context RHidentity")
	}
	identity := indent.Identity
	// ensure identity org_id is the same as the update transaction
	if identity.OrgID != update.OrgID {
		s.log.Error("context identity org_id and update transaction org_id mismatch")
		return nil, ErrOrgIDMismatch
	}
	// 3. Loop through all devices in UpdateTransaction
	dispatchRecords := update.DispatchRecords
	for _, device := range update.Devices {
		device := device // this will prevent implicit memory aliasing in the loop
		// Create new &DispatcherPayload{}
		payloadDispatcher := playbookdispatcher.DispatcherPayload{
			Recipient:    device.RHCClientID,
			PlaybookURL:  playbookURL,
			OrgID:        update.OrgID,
			PlaybookName: "Edge-management",
			Principal:    identity.User.Username,
		}
		s.log.Debug("Calling playbook dispatcher")
		exc, err := s.PlaybookClient.ExecuteDispatcher(payloadDispatcher)

		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error on playbook-dispatcher execution")
			update.Status = models.UpdateStatusError

			update.DispatchRecords = append(dispatchRecords, models.DispatchRecord{
				DeviceID:    device.ID,
				PlaybookURL: playbookURL,
				Status:      models.DispatchRecordStatusError,
				Reason:      models.UpdateReasonFailure,
			})
			db.DB.Omit("Devices.*").Debug().Save(update)
			return nil, err
		}

		for _, excPlaybook := range exc {
			if excPlaybook.StatusCode == http.StatusCreated {
				device.Connected = true
				dispatchRecord := &models.DispatchRecord{
					Device:               &device,
					DeviceID:             device.ID,
					PlaybookURL:          playbookURL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				device.Connected = false
				dispatchRecord := &models.DispatchRecord{
					Device:      &device,
					DeviceID:    device.ID,
					PlaybookURL: playbookURL,
					Status:      models.DispatchRecordStatusError,
					Reason:      models.UpdateReasonFailure,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			}
			db.DB.Save(&device)
		}
		update.DispatchRecords = dispatchRecords
		err = s.SetUpdateStatus(update)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error saving update")
			return nil, err
		}
		dRecord := db.DB.Debug().Omit("Devices, DispatchRecords.Device").Save(update)
		if dRecord.Error != nil {
			s.log.WithField("error", dRecord.Error).Error("Error saving Dispach Record")
			return nil, dRecord.Error
		}

	}

	s.log.WithField("updateID", update.ID).Info("Update was finished")
	return update, nil
}

// GetUpdatePlaybook is the function that returns the path to an update playbook
func (s *UpdateService) GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error) {
	// TODO change this path name to use org id
	fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", update.OrgID, update.ID)
	path := fmt.Sprintf("%s/playbooks/%s", update.OrgID, fname)
	return s.FilesService.GetFile(path)
}

func (s *UpdateService) getPlaybookURL(updateID uint) string {
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/edge/v1/updates/%d/update-playbook.yml",
		cfg.EdgeAPIBaseURL, updateID)
	return url
}

// WriteTemplate is the function that writes the template to a file
func (s *UpdateService) WriteTemplate(templateInfo TemplateRemoteInfo, orgID string) (string, error) {
	cfg := config.Get()
	filePath := cfg.TemplatesPath
	templateName := "template_playbook_dispatcher_ostree_upgrade_payload.yml"
	templateContents, err := template.New(templateName).Delims("@@", "@@").ParseFiles(filePath + templateName)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error parsing playbook template")
		return "", err
	}

	edgeCertAPIBaseURL, err := url.Parse(cfg.EdgeCertAPIBaseURL)
	if err != nil {
		s.log.WithFields(log.Fields{"error": err.Error(), "url": cfg.EdgeCertAPIBaseURL}).Error("error while parsing config edge cert api url")
		return "", err
	}
	repoURL := fmt.Sprintf("%s://%s/api/edge/v1/storage/update-repos/%d", edgeCertAPIBaseURL.Scheme, edgeCertAPIBaseURL.Host, templateInfo.UpdateTransactionID)

	templateData := playbooks{
		GoTemplateRemoteName: templateInfo.RemoteName,
		UpdateNumber:         strconv.FormatUint(uint64(templateInfo.UpdateTransactionID), 10),
		RepoURL:              repoURL,
		RepoContentURL:       fmt.Sprintf("%s/content", repoURL),
		RemoteOstreeUpdate:   templateInfo.RemoteOstreeUpdate,
		OSTreeRef:            templateInfo.OSTreeRef,
	}

	// TODO change the same time as line 231
	fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", orgID, templateInfo.UpdateTransactionID)
	tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", orgID, fname)
	dirpath := fmt.Sprintf("/tmp/v2/%s", orgID)

	// create the full path for /tmp/v2/<orgID>
	if err := os.MkdirAll(dirpath, 0770); err != nil {
		s.log.WithField("error", err.Error()).Errorf("Error creating folder: %s", dirpath)
		return "", err
	}
	// create the tmpfile with the full path
	f, err := os.Create(tmpfilepath)
	if err != nil {
		s.log.WithField("error", err.Error()).Errorf("Error creating file: %s", tmpfilepath)
		return "", err
	}
	err = templateContents.Execute(f, templateData)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error executing template")
		return "", err
	}

	uploadPath := fmt.Sprintf("%s/playbooks/%s", orgID, fname)
	playbookURL, err := s.FilesService.GetUploader().UploadFile(tmpfilepath, uploadPath)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error uploading file to S3")
		return "", err
	}
	s.log.WithField("playbookURL", playbookURL).Info("Template file uploaded to S3")
	err = os.Remove(tmpfilepath)
	if err != nil {
		// TODO: Fail silently, find a way to create alerts based on this log
		// The container will end up out of space if we don't fix it in the long run.
		s.log.WithField("error", err.Error()).Error("Error deleting temp file")
	}
	playbookURL = s.getPlaybookURL(templateInfo.UpdateTransactionID)
	s.log.WithField("playbookURL", playbookURL).Info("Proxied playbook URL")
	s.log.Info("Update was finished")
	return playbookURL, nil
}

// GetUpdateTransactionsForDevice returns all update transactions for a given device
func (s *UpdateService) GetUpdateTransactionsForDevice(device *models.Device) (*[]models.UpdateTransaction, error) {
	var updates []models.UpdateTransaction
	result := db.DB.Debug().
		Table("update_transactions").
		Preload("DispatchRecords", func(db *gorm.DB) *gorm.DB {
			return db.Where("dispatch_records.device_id = ?", device.ID)
		}).
		Joins(
			`JOIN updatetransaction_devices ON update_transactions.id = updatetransaction_devices.update_transaction_id`).
		Where(`updatetransaction_devices.device_id = ?`,
			device.ID,
		).Group("id").Order("created_at DESC").Limit(10).Find(&updates)
	if result.Error != nil {
		return nil, result.Error
	}
	return &updates, nil
}

// Status defined by https://github.com/RedHatInsights/playbook-dispatcher/blob/master/schema/run.event.yaml
const (
	// PlaybookStatusRunning is the status when a playbook is still running
	PlaybookStatusRunning = "running"
	// PlaybookStatusSuccess is the status when a playbook has run successfully
	PlaybookStatusSuccess = "success"
	// PlaybookStatusFailure is the status when a playbook execution fails
	PlaybookStatusFailure = "failure"
	// PlaybookStatusTimeout is the status when a playbook execution times out
	PlaybookStatusTimeout = "timeout"
)

// ProcessPlaybookDispatcherRunEvent is the method that processes messages from playbook dispatcher to set update statuses
func (s *UpdateService) ProcessPlaybookDispatcherRunEvent(message []byte) error {
	var e *PlaybookDispatcherEvent
	err := json.Unmarshal(message, &e)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error unmarshaling playbook dispatcher event message")
		return err
	}
	s.log = log.WithFields(log.Fields{
		"PlaybookDispatcherID": e.Payload.ID,
		"Status":               e.Payload.Status,
	})
	if e.Payload.Status == PlaybookStatusRunning {
		s.log.Debug("Playbook is running - waiting for next messages")
		return nil
	} else if e.Payload.Status == PlaybookStatusSuccess {
		s.log.Debug("The playbook was applied successfully. Waiting two minutes for reboot before setting status to success.")
		time.Sleep(s.WaitForReboot)
	}

	var dispatchRecord models.DispatchRecord
	result := db.DB.Where(&models.DispatchRecord{PlaybookDispatcherID: e.Payload.ID}).Preload("Device").First(&dispatchRecord)
	if result.Error != nil {
		return result.Error
	}

	switch e.Payload.Status {
	case PlaybookStatusSuccess:
		// TODO: We might wanna check if it's really success by checking the running hash on the device here
		dispatchRecord.Status = models.DispatchRecordStatusComplete
		dispatchRecord.Device.AvailableHash = os.DevNull
		dispatchRecord.Device.CurrentHash = dispatchRecord.Device.AvailableHash
	case PlaybookStatusRunning:
		dispatchRecord.Status = models.DispatchRecordStatusRunning
	case PlaybookStatusTimeout:
		dispatchRecord.Status = models.DispatchRecordStatusError
		dispatchRecord.Reason = models.UpdateReasonTimeout
	case PlaybookStatusFailure:
		dispatchRecord.Status = models.DispatchRecordStatusError
		dispatchRecord.Reason = models.UpdateReasonFailure
	default:
		dispatchRecord.Status = models.DispatchRecordStatusError
		dispatchRecord.Reason = models.UpdateReasonFailure
		s.log.Error("Playbook status is not on the json schema for this event")
	}

	result = db.DB.Debug().Omit("Device").Save(&dispatchRecord)
	if result.Error != nil {
		return result.Error
	}

	return s.SetUpdateStatusBasedOnDispatchRecord(dispatchRecord)
}

// SetUpdateStatusBasedOnDispatchRecord is the function that, given a dispatch record, finds the update transaction related to and update its status if necessary
func (s *UpdateService) SetUpdateStatusBasedOnDispatchRecord(dispatchRecord models.DispatchRecord) error {
	var update models.UpdateTransaction
	result := db.DB.Table("update_transactions").Preload("DispatchRecords").
		Joins(`JOIN updatetransaction_dispatchrecords ON update_transactions.id = updatetransaction_dispatchrecords.update_transaction_id`).
		Where(`updatetransaction_dispatchrecords.dispatch_record_id = ?`, dispatchRecord.ID).First(&update)
	if result.Error != nil {
		log.WithError(result.Error)
		return result.Error
	}

	if err := s.SetUpdateStatus(&update); err != nil {
		return err
	}

	return s.UpdateDevicesFromUpdateTransaction(update)
}

// SetUpdateStatus is the function to set the update status from an UpdateTransaction
func (s *UpdateService) SetUpdateStatus(update *models.UpdateTransaction) error {
	allSuccess := true

	for _, d := range update.DispatchRecords {
		if d.Status != models.DispatchRecordStatusComplete {
			allSuccess = false
		}
		if d.Status == models.DispatchRecordStatusError {
			update.Status = models.UpdateStatusError
			break
		}
	}
	if allSuccess {
		update.Status = models.UpdateStatusSuccess
	}
	// If there isn't an error and it's not all success, some updates are still happening
	result := db.DB.Debug().Model(&models.UpdateTransaction{}).Where("ID=?", update.ID).Update("Status", update.Status)

	return result.Error
}

// SendDeviceNotification connects to platform.notifications.ingress on image topic
func (s *UpdateService) SendDeviceNotification(i *models.UpdateTransaction) (ImageNotification, error) {
	s.log.WithField("message", i).Info("SendImageNotification::Starts")
	var notify ImageNotification
	notify.Version = NotificationConfigVersion
	notify.Bundle = NotificationConfigBundle
	notify.Application = NotificationConfigApplication
	notify.EventType = NotificationConfigEventTypeDevice
	notify.Timestamp = time.Now().Format(time.RFC3339)

	if clowder.IsClowderEnabled() {
		var users []string
		var events []EventNotification
		var event EventNotification
		var recipients []RecipientNotification
		var recipient RecipientNotification
		brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))

		for i, b := range clowder.LoadedConfig.Kafka.Brokers {
			brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
			fmt.Println(brokers[i])
		}

		topic := NotificationTopic

		// Create Producer instance
		p, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers": brokers[0]})
		if err != nil {
			s.log.WithField("message", err.Error()).Error("Error creating Kafka producer")
			os.Exit(1)
		}

		type metadata struct {
			metaMap map[string]string
		}
		emptyJSON := metadata{
			metaMap: make(map[string]string),
		}

		event.Metadata = emptyJSON.metaMap

		event.Payload = fmt.Sprintf("{  \"UpdateID\" : \"%v\"}", i.ID)
		events = append(events, event)

		recipient.IgnoreUserPreferences = false
		recipient.OnlyAdmins = false
		users = append(users, NotificationConfigUser)
		recipient.Users = users
		recipients = append(recipients, recipient)

		notify.OrgID = i.OrgID
		notify.Context = fmt.Sprintf("{  \"CommitID\" : \"%v\"}", i.CommitID)
		notify.Events = events
		notify.Recipients = recipients

		// assemble the message to be sent
		recordKey := "ImageCreationStarts"
		recordValue, _ := json.Marshal(notify)

		s.log.WithField("message", recordValue).Info("Preparing record for producer")

		// send the message
		perr := p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Key:            []byte(recordKey),
			Value:          []byte(recordValue),
		}, nil)

		if perr != nil {
			s.log.WithField("message", perr.Error()).Error("Error on produce")
			return notify, err
		}
		p.Close()
		s.log.WithField("message", topic).Info("SendNotification message was produced to topic")
		fmt.Printf("SendNotification message was produced to topic %s!\n", topic)
		return notify, nil
	}
	return notify, nil
}

// UpdateDevicesFromUpdateTransaction update device with new image and update availability
func (s *UpdateService) UpdateDevicesFromUpdateTransaction(update models.UpdateTransaction) error {
	logger := s.log.WithFields(log.Fields{"org_id": update.OrgID, "context": "UpdateDevicesFromUpdateTransaction"})
	if update.Status != models.UpdateStatusSuccess {
		// update only when update is successful
		// do nothing
		logger.Debug("ignore device update when update is not successful")
		return nil
	}

	// reload update transaction from db
	var currentUpdate models.UpdateTransaction
	if result := db.Org(update.OrgID, "").Preload("Devices").Preload("Commit").First(&currentUpdate, update.ID); result.Error != nil {
		return result.Error
	}

	if currentUpdate.Commit == nil {
		logger.Error("The update transaction has no commit defined")
		return ErrUndefinedCommit
	}

	// get the update commit image
	var deviceImage models.Image
	if result := db.Org(currentUpdate.OrgID, "images").
		Joins("JOIN commits ON commits.id = images.commit_id").
		Where("commits.os_tree_commit = ? ", currentUpdate.Commit.OSTreeCommit).
		First(&deviceImage); result.Error != nil {
		logger.WithField("error", result.Error).Error("Error while getting device image")
		return result.Error
	}

	// get image update availability, by finding if there is later images updates
	// consider only those with ImageStatusSuccess
	var updateImages []models.Image
	if result := db.Org(deviceImage.OrgID, "").Select("id").Where("image_set_id = ? AND status = ? AND created_at > ?",
		deviceImage.ImageSetID, models.ImageStatusSuccess, deviceImage.CreatedAt).Find(&updateImages); result.Error != nil {
		logger.WithField("error", result.Error).Error("Error while getting update images")
		return result.Error
	}
	updateAvailable := len(updateImages) > 0

	// create a slice of devices ids
	devicesIDS := make([]uint, 0, len(currentUpdate.Devices))
	for _, device := range currentUpdate.Devices {
		devicesIDS = append(devicesIDS, device.ID)
	}

	// update devices with image and update availability
	if result := db.Org(deviceImage.OrgID, "").Model(&models.Device{}).Where("id IN (?) ", devicesIDS).
		Updates(map[string]interface{}{"image_id": deviceImage.ID, "update_available": updateAvailable}); result.Error != nil {
		logger.WithField("error", result.Error).Error("Error occurred while updating device image and update_available")
		return result.Error
	}

	return nil
}

// ValidateUpdateSelection validate the images for update
func (s *UpdateService) ValidateUpdateSelection(orgID string, imageIds []uint) (bool, error) {
	var count int64
	if result := db.Org(orgID, "").Debug().Table("images").Where(`id IN ?`, imageIds).Group("image_set_id").Count(&count); result.Error != nil {
		return false, result.Error
	}

	return count == 1, nil
}

// ValidateUpdateDeviceGroup validate the devices on device group for update
func (s *UpdateService) ValidateUpdateDeviceGroup(orgID string, deviceGroupID uint) (bool, error) {
	var count int64

	if result := db.Org(orgID, "Device_Groups").Model(&models.DeviceGroup{}).Where(`Device_Groups.id = ?`, deviceGroupID).
		Joins(`JOIN Device_Groups_Devices  ON Device_Groups.id = Device_Groups_Devices.device_group_id`).
		Joins(`JOIN Devices  ON Device_Groups_Devices.device_id = Devices.id`).
		Where("Devices.image_id IS NOT NULL AND Devices.image_id != 0").
		Joins(`JOIN Images  ON Devices.image_id = Images.id`).
		Group("image_set_id").Count(&count); result.Error != nil {
		return false, result.Error
	}

	return count == 1, nil
}

// BuildUpdateTransactions build records
func (s *UpdateService) BuildUpdateTransactions(devicesUpdate *models.DevicesUpdate,
	orgID string, commit *models.Commit) (*[]models.UpdateTransaction, error) {
	var inv inventory.Response
	var ii []inventory.Response
	var err error

	if len(devicesUpdate.DevicesUUID) > 0 {
		for _, UUID := range devicesUpdate.DevicesUUID {
			inv, err = s.Inventory.ReturnDevicesByID(UUID)
			if inv.Count > 0 {
				ii = append(ii, inv)
			}
			if err != nil {
				err := errors.NewNotFound(fmt.Sprintf("No devices found for UUID %s", UUID))
				return nil, err
			}
		}
	}

	s.log.WithField("inventoryDevice", inv).Debug("Device retrieved from inventory")
	var updates []models.UpdateTransaction
	for _, inventory := range ii {
		// Create the models.UpdateTransaction
		update := models.UpdateTransaction{
			OrgID:    orgID,
			CommitID: devicesUpdate.CommitID,
			Status:   models.UpdateStatusCreated,
		}

		// Get the models.Commit from the Commit ID passed in via JSON
		update.Commit = commit

		notify, errNotify := s.SendDeviceNotification(&update)
		if errNotify != nil {
			s.log.WithField("message", errNotify.Error()).Error("Error to send device notification")
			s.log.WithField("message", notify).Error("Notify Error")

		}

		update.DispatchRecords = []models.DispatchRecord{}

		devices := update.Devices
		oldCommits := update.OldCommits
		toUpdate := true

		var repo *models.Repo

		for _, device := range inventory.Result {
			//  Check for the existence of a Repo that already has this commit and don't duplicate
			var updateDevice *models.Device
			dbDevice := db.DB.Where("uuid = ?", device.ID).First(&updateDevice)
			if dbDevice.Error != nil {
				if !(dbDevice.Error.Error() == "Device was not found") {
					s.log.WithFields(log.Fields{
						"error":      dbDevice.Error.Error(),
						"deviceUUID": device.ID,
					}).Error("Error retrieving device record from database")
					err = errors.NewBadRequest(err.Error())
					return nil, err
				}
				s.log.WithFields(log.Fields{
					"error":      dbDevice.Error.Error(),
					"deviceUUID": device.ID,
				}).Info("Creating a new device on the database")
				updateDevice = &models.Device{
					UUID:  device.ID,
					OrgID: orgID,
				}
				if result := db.DB.Debug().Omit("Devices.*").Create(&updateDevice); result.Error != nil {
					return nil, result.Error
				}
			}

			if device.Ostree.RHCClientID == "" {
				s.log.WithFields(log.Fields{
					"deviceUUID": device.ID,
				}).Info("Device is disconnected")
				update.Status = models.UpdateStatusDeviceDisconnected
				update.Devices = append(update.Devices, *updateDevice)
				if result := db.DB.Debug().Omit("Devices.*").Create(&update); result.Error != nil {
					return nil, result.Error
				}
				continue
			}
			updateDevice.RHCClientID = device.Ostree.RHCClientID
			updateDevice.AvailableHash = update.Commit.OSTreeCommit

			// update the device orgID if undefined
			if updateDevice.OrgID == "" {
				updateDevice.OrgID = orgID
			}
			result := db.DB.Debug().Omit("Devices.*").Save(&updateDevice)
			if result.Error != nil {
				return nil, result.Error
			}

			s.log.WithFields(log.Fields{
				"updateDevice": updateDevice,
			}).Debug("Saved updated device")

			devices = append(devices, *updateDevice)
			update.Devices = devices

			for _, deployment := range device.Ostree.RpmOstreeDeployments {
				s.log.WithFields(log.Fields{
					"ostreeDeployment": deployment,
				}).Debug("Got ostree deployment for device")

				if deployment.Booted {
					s.log.WithFields(log.Fields{
						"booted": deployment.Booted,
					}).Debug("device has been booted")

					if commit.OSTreeCommit == deployment.Checksum {
						toUpdate = false
						break
					}
					var oldCommit models.Commit
					result := db.DB.Where("os_tree_commit = ?", deployment.Checksum).First(&oldCommit)
					if result.Error != nil {
						if result.Error.Error() != "record not found" {
							s.log.WithField("error", err.Error()).Error("Error returning old commit for this ostree checksum")
							err := errors.NewBadRequest(err.Error())

							return nil, err
						}
					}
					if result.RowsAffected == 0 {
						s.log.Debug("No old commits found")
					} else {
						if !contains(oldCommits, oldCommit) {
							oldCommits = append(oldCommits, oldCommit)
						}
					}
					currentImage, cError := s.ImageService.GetImageByOSTreeCommitHash(deployment.Checksum)
					if cError != nil {
						s.log.WithField("error", err.Error()).Error("Error returning current image ostree checksum")

					}
					updatedImage, uError := s.ImageService.GetImageByOSTreeCommitHash(commit.OSTreeCommit)
					if uError != nil {
						s.log.WithField("error", err.Error()).Error("Error returning current image ostree checksum")
					}
					if config.DistributionsRefs[currentImage.Distribution] != config.DistributionsRefs[updatedImage.Distribution] {
						update.ChangesRefs = true
					}
				}
			}

			if toUpdate {
				if repo == nil {
					//  Removing commit dependency to avoid overwriting the repo
					s.log.WithField("updateID", update.ID).Debug("Creating new repo for update transaction")
					repo = &models.Repo{
						Status: models.RepoStatusBuilding,
					}
					result := db.DB.Create(&repo)
					if result.Error != nil {
						s.log.WithField("error", result.Error.Error()).Debug("Result error")
					}
					s.log.WithFields(log.Fields{
						"repoURL": repo.URL,
						"repoID":  repo.ID,
					}).Debug("Getting repo info")
				}

				update.Repo = repo

				// Should not create a transaction to device already updated
				update.OldCommits = oldCommits
				update.RepoID = &repo.ID
				if err := db.DB.Omit("Devices.*").Save(&update).Error; err != nil {
					err = errors.NewBadRequest(err.Error())
					s.log.WithField("error", err.Error()).Error("Error encoding error")
					return nil, err
				}
			}

		}
		if toUpdate {
			updates = append(updates, update)
		}
		s.log.WithField("updateID", update.ID).Info("Update has been created")
	}
	return &updates, nil
}

func contains(oldCommits []models.Commit, searchCommit models.Commit) bool {
	for _, commit := range oldCommits {
		if commit.ID == searchCommit.ID {
			return true
		}
	}
	return false
}
