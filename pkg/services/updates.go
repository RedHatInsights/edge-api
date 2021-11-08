package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/template"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// UpdateServiceInterface defines the interface that helps
// handle the business logic of sending updates to a edge device
type UpdateServiceInterface interface {
	CreateUpdate(id uint) (*models.UpdateTransaction, error)
	GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error)
	GetUpdateTransactionsForDevice(device *models.Device) (*[]models.UpdateTransaction, error)
	ProcessPlaybookDispatcherRunEvent(message []byte) error
}

// NewUpdateService gives a instance of the main implementation of a UpdateServiceInterface
func NewUpdateService(ctx context.Context) UpdateServiceInterface {
	return &UpdateService{
		Context:      ctx,
		FilesService: NewFilesService(),
		RepoBuilder:  NewRepoBuilder(ctx),
	}
}

// UpdateService is the main implementation of a UpdateServiceInterface
type UpdateService struct {
	Context      context.Context
	RepoBuilder  RepoBuilderInterface
	FilesService FilesService
}

type playbooks struct {
	GoTemplateRemoteName string
	GoTemplateRemoteURL  string
	GoTemplateContentURL string
	GoTemplateGpgVerify  string
	OstreeRemoteName     string
	OstreeRemoteURL      string
	OstreeContentURL     string
	OstreeGpgVerify      string
	OstreeGpgKeypath     string
	OstreeRemoteTemplate string
}

// TemplateRemoteInfo the values to playbook
type TemplateRemoteInfo struct {
	RemoteName          string
	RemoteURL           string
	ContentURL          string
	GpgVerify           string
	UpdateTransactionID uint
}

// PlaybookDispatcherEventPayload belongs to PlaybookDispatcherEvent
type PlaybookDispatcherEventPayload struct {
	ID            string `json:"id"`
	Account       string `json:"account"`
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

// CreateUpdate is the function that creates an update transaction
func (s *UpdateService) CreateUpdate(id uint) (*models.UpdateTransaction, error) {
	var update *models.UpdateTransaction
	db.DB.Preload("DispatchRecords").Preload("Devices").Joins("Commit").Joins("Repo").Find(&update, id)
	update.Status = models.UpdateStatusBuilding
	db.DB.Save(&update)

	WaitGroup.Add(1) // Processing one update
	defer func() {
		WaitGroup.Done() // Done with one update (sucessfuly or not)
		log.Debug("Done with one update - sucessfuly or not")
		if err := recover(); err != nil {
			log.Fatalf("%s", err)
		}
	}()
	go func(update *models.UpdateTransaction) {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		sig := <-sigint
		// Reload update to get updated status
		db.DB.First(&update, update.ID)
		if update.Status == models.UpdateStatusBuilding {
			log.WithFields(log.Fields{
				"signal":   sig,
				"updateID": update.ID,
			}).Info("Captured signal marking update as error")
			update.Status = models.UpdateStatusError
			tx := db.DB.Save(update)
			if tx.Error != nil {
				log.Fatal("Error saving update: %s ", tx.Error.Error())
			}
			WaitGroup.Done()
		}
	}(update)

	update, err := s.RepoBuilder.BuildUpdateRepo(id)
	if err != nil {
		db.DB.First(&update, id)
		update.Status = models.UpdateStatusError
		db.DB.Save(update)
		log.Error(err)
		return nil, err
	}
	// FIXME - implement playbook dispatcher scheduling
	// 1. Create template Playbook
	// 2. Upload templated playbook
	var remoteInfo TemplateRemoteInfo
	remoteInfo.RemoteURL = update.Repo.URL
	remoteInfo.RemoteName = "rhel-edge"
	remoteInfo.ContentURL = update.Repo.URL
	remoteInfo.UpdateTransactionID = update.ID
	remoteInfo.GpgVerify = "false"
	playbookURL, err := s.writeTemplate(remoteInfo, update.Account)
	if err != nil {
		update.Status = models.UpdateStatusError
		db.DB.Save(update)
		log.Error(err)
		return nil, err
	}
	// 3. Loop through all devices in UpdateTransaction
	dispatchRecords := update.DispatchRecords
	for _, device := range update.Devices {
		var updateDevice *models.Device
		result := db.DB.Where("uuid = ?", device.UUID).First(&updateDevice)
		if err != nil {
			log.Errorf("Error on GetDeviceByUUID: %#v ", result.Error.Error())
			return nil, result.Error
		}
		// Create new &DispatcherPayload{}
		payloadDispatcher := playbookdispatcher.DispatcherPayload{
			Recipient:   device.RHCClientID,
			PlaybookURL: playbookURL,
			Account:     update.Account,
		}
		log.Infof("Call Execute Dispatcher: : %#v", payloadDispatcher)
		client := playbookdispatcher.InitClient(s.Context)
		exc, err := client.ExecuteDispatcher(payloadDispatcher)

		if err != nil {
			log.Errorf("Error on playbook-dispatcher execution: %#v ", err)
			return nil, err
		}
		for _, excPlaybook := range exc {
			if excPlaybook.StatusCode == http.StatusCreated {
				device.Connected = true
				dispatchRecord := &models.DispatchRecord{
					Device:               updateDevice,
					PlaybookURL:          playbookURL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				device.Connected = false
				dispatchRecord := &models.DispatchRecord{
					Device:      updateDevice,
					PlaybookURL: playbookURL,
					Status:      models.DispatchRecordStatusError,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			}

		}
		update.DispatchRecords = dispatchRecords
		tx := db.DB.Save(&update)
		if tx.Error != nil {
			log.Errorf("Error saving update: %s ", tx.Error.Error())
			return nil, err
		}
	}

	log.Infof("Update was finished for :: %d", update.ID)
	return update, nil
}

// GetUpdatePlaybook is the function that returns the path to an update playbook
func (s *UpdateService) GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error) {
	fname := fmt.Sprintf("playbook_dispatcher_update_%d.yml", update.ID)
	path := fmt.Sprintf("%s/playbooks/%s", update.Account, fname)
	return s.FilesService.GetFile(path)
}

func (s *UpdateService) getPlaybookURL(updateID uint) string {
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/edge/v1/updates/%d/update-playbook.yml",
		cfg.EdgeAPIBaseURL, updateID)
	return url
}

func (s *UpdateService) writeTemplate(templateInfo TemplateRemoteInfo, account string) (string, error) {
	cfg := config.Get()
	filePath := cfg.TemplatesPath
	templateName := "template_playbook_dispatcher_ostree_upgrade_payload.yml"
	template, err := template.ParseFiles(filePath + templateName)
	if err != nil {
		log.Errorf("Error parsing playbook template  :: %s", err.Error())
		return "", err
	}
	templateData := playbooks{
		GoTemplateRemoteName: templateInfo.RemoteName,
		GoTemplateRemoteURL:  templateInfo.RemoteURL,
		GoTemplateContentURL: templateInfo.ContentURL,
		GoTemplateGpgVerify:  templateInfo.GpgVerify,
		OstreeRemoteName:     "{{ ostree_remote_name }}",
		OstreeRemoteURL:      "{{ ostree_remote_url }}",
		OstreeContentURL:     "{{ ostree_content_url }}",
		OstreeGpgVerify:      "false",
		OstreeGpgKeypath:     "/etc/pki/rpm-gpg/",
		OstreeRemoteTemplate: "{{ ostree_remote_template }}"}

	fname := fmt.Sprintf("playbook_dispatcher_update_%d.yml", templateInfo.UpdateTransactionID)
	tmpfilepath := fmt.Sprintf("/tmp/%s", fname)
	f, err := os.Create(tmpfilepath)
	if err != nil {
		log.Errorf("Error creating file: %s", err.Error())
		return "", err
	}
	err = template.Execute(f, templateData)
	if err != nil {
		log.Errorf("Error executing template: %s ", err.Error())
		return "", err
	}

	uploadPath := fmt.Sprintf("%s/playbooks/%s", account, fname)
	playbookURL, err := s.FilesService.GetUploader().UploadFile(tmpfilepath, uploadPath)
	if err != nil {
		log.Errorf("Error uploading file to S3: %s ", err.Error())
		return "", err
	}
	log.Infof("Template file uploaded to S3, URL: %s", playbookURL)
	err = os.Remove(tmpfilepath)
	if err != nil {
		// TODO: Fail silently, find a way to create alerts based on this log
		// The container will end up out of space if we don't fix it in the long run.
		log.Errorf("Error deleting temp file: %s ", err.Error())
	}
	playbookURL = s.getPlaybookURL(templateInfo.UpdateTransactionID)
	log.Infof("Proxied playbook URL: %s", playbookURL)
	log.Infof("::WriteTemplate: ENDs")
	return playbookURL, nil
}

// GetUpdateTransactionsForDevice returns all update transactions for a given device
func (s *UpdateService) GetUpdateTransactionsForDevice(device *models.Device) (*[]models.UpdateTransaction, error) {
	var updates []models.UpdateTransaction
	result := db.DB.
		Table("update_transactions").
		Joins(
			`JOIN updatetransaction_devices ON update_transactions.id = updatetransaction_devices.update_transaction_id`).
		Where(`updatetransaction_devices.device_id = ?`,
			device.ID,
		).Group("id").Find(&updates)
	if result.Error != nil {
		return nil, result.Error
	}
	return &updates, nil
}

// Status defined by https://github.com/RedHatInsights/playbook-dispatcher/blob/master/schema/run.event.yaml
const (
	// PlaybookStatusRunning is the status when a playbook is still running
	PlaybookStatusRunning = "running"
	// PlaybookStatusSuccess is the status when a playbook has run sucessfully
	PlaybookStatusSuccess = "success"
	// PlaybookStatusFailure is the status when a playbook execution fails
	PlaybookStatusFailure = "failure"
	// PlaybookStatusFailure is the status when a playbook execution times out
	PlaybookStatusTimeout = "timeout"
)

// ProcessPlaybookDispatcherRunEvent is the method that processes messages from playbook dispatcher to set update statuses
func (s *UpdateService) ProcessPlaybookDispatcherRunEvent(message []byte) error {
	var e *PlaybookDispatcherEvent
	err := json.Unmarshal(message, &e)
	if err != nil {
		return err
	}
	log := log.WithFields(log.Fields{
		"PlaybookDispatcherID": e.Payload.ID,
		"Status":               e.Payload.Status,
	})
	if e.Payload.Status == PlaybookStatusRunning {
		log.Debug("Playbook is running")
		return nil
	}

	var dispatchRecord models.DispatchRecord
	result := db.DB.Where(&models.DispatchRecord{PlaybookDispatcherID: e.Payload.ID}).First(&dispatchRecord)
	if result.Error != nil {
		return result.Error
	}

	if e.Payload.Status == PlaybookStatusFailure || e.Payload.Status == PlaybookStatusTimeout {
		dispatchRecord.Status = models.DispatchRecordStatusError
	} else if e.Payload.Status == PlaybookStatusSuccess {
		dispatchRecord.Status = models.DispatchRecordStatusComplete
	} else if e.Payload.Status == PlaybookStatusRunning {
		dispatchRecord.Status = models.DispatchRecordStatusRunning
	} else {
		log.Fatal("Playbook status is not on the json schema for this event")
	}
	result = db.DB.Save(&dispatchRecord)
	if result.Error != nil {
		return result.Error
	}

	return s.SetUpdateStatusBasedOnDispatchRecord(dispatchRecord)
}

// SetUpdateStatusBasedOnDispatchRecord is the function that, given a dispatch record, finds the update transaction related to and update its status if necessary
func (s *UpdateService) SetUpdateStatusBasedOnDispatchRecord(dispatchRecord models.DispatchRecord) error {
	var update models.UpdateTransaction
	result := db.DB.
		Table("update_transactions").
		Joins(
			`JOIN updatetransaction_dispatchrecords ON update_transactions.id = updatetransaction_dispatchrecords.update_transaction_id`).
		Where(`updatetransaction_dispatchrecords.dispatch_record_id = ?`,
			dispatchRecord.ID,
		).First(&update)
	if result.Error != nil {
		return result.Error
	}

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
	result = db.DB.Save(&update)
	return result.Error
}
