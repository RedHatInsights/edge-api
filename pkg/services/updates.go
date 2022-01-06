package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	WriteTemplate(templateInfo TemplateRemoteInfo, account string) (string, error)
	SetUpdateStatusBasedOnDispatchRecord(dispatchRecord models.DispatchRecord) error
	SetUpdateStatus(update *models.UpdateTransaction) error
}

// NewUpdateService gives a instance of the main implementation of a UpdateServiceInterface
func NewUpdateService(ctx context.Context, log *log.Entry) UpdateServiceInterface {
	return &UpdateService{
		Service:       Service{ctx: ctx, log: log.WithField("service", "update")},
		FilesService:  NewFilesService(),
		RepoBuilder:   NewRepoBuilder(ctx, log),
		WaitForReboot: time.Minute * 5,
	}
}

// UpdateService is the main implementation of a UpdateServiceInterface
type UpdateService struct {
	Service
	RepoBuilder   RepoBuilderInterface
	FilesService  FilesService
	WaitForReboot time.Duration
}

type playbooks struct {
	GoTemplateRemoteName string
	GoTemplateGpgVerify  string
	OstreeRemoteName     string
	OstreeGpgVerify      string
	OstreeGpgKeypath     string
	FleetInfraEnv        string
	UpdateNumber         string
	RepoURL              string
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
		WaitGroup.Done() // Done with one update (successfuly or not)
		s.log.Debug("Done with one update - successfuly or not")
		if err := recover(); err != nil {
			s.log.WithField("error", err).Fatal("Error on update")
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
				s.log.WithField("error", tx.Error.Error()).Fatal("Error saving update")
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
	playbookURL, err := s.WriteTemplate(remoteInfo, update.Account)
	if err != nil {
		update.Status = models.UpdateStatusError
		db.DB.Save(update)
		s.log.WithField("error", err.Error()).Error("Error writing playbook template")
		return nil, err
	}
	// 3. Loop through all devices in UpdateTransaction
	dispatchRecords := update.DispatchRecords
	for _, device := range update.Devices {
		// Create new &DispatcherPayload{}
		payloadDispatcher := playbookdispatcher.DispatcherPayload{
			Recipient:   device.RHCClientID,
			PlaybookURL: playbookURL,
			Account:     update.Account,
		}
		s.log.Debug("Calling playbook dispatcher")
		client := playbookdispatcher.InitClient(s.ctx)
		exc, err := client.ExecuteDispatcher(payloadDispatcher)

		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error on playbook-dispatcher execution")
			update.Status = models.UpdateStatusError
			db.DB.Save(update)
			return nil, err
		}
		for _, excPlaybook := range exc {
			if excPlaybook.StatusCode == http.StatusCreated {
				device.Connected = true
				dispatchRecord := &models.DispatchRecord{
					Device:               &device,
					PlaybookURL:          playbookURL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				device.Connected = false
				dispatchRecord := &models.DispatchRecord{
					Device:      &device,
					PlaybookURL: playbookURL,
					Status:      models.DispatchRecordStatusError,
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
	}

	s.log.WithField("updateID", update.ID).Info("Update was finished")
	return update, nil
}

// GetUpdatePlaybook is the function that returns the path to an update playbook
func (s *UpdateService) GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error) {
	fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", update.Account, update.ID)
	path := fmt.Sprintf("%s/playbooks/%s", update.Account, fname)
	return s.FilesService.GetFile(path)
}

func (s *UpdateService) getPlaybookURL(updateID uint) string {
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/edge/v1/updates/%d/update-playbook.yml",
		cfg.EdgeAPIBaseURL, updateID)
	return url
}

// WriteTemplate is the function that writes the template to a file
func (s *UpdateService) WriteTemplate(templateInfo TemplateRemoteInfo, account string) (string, error) {
	cfg := config.Get()
	filePath := cfg.TemplatesPath
	templateName := "template_playbook_dispatcher_ostree_upgrade_payload.yml"
	templateContents, err := template.New(templateName).Delims("@@", "@@").ParseFiles(filePath + templateName)
	if err != nil {
		s.log.WithField("error", err.Error()).Errorf("Error parsing playbook template")
		return "", err
	}
	var envName string
	if strings.Contains(cfg.BucketName, "-prod") || strings.Contains(cfg.BucketName, "-stage") || strings.Contains(cfg.BucketName, "-perf") {
		bucketNameSplit := strings.Split(cfg.BucketName, "-")
		envName = bucketNameSplit[len(bucketNameSplit)-1]
	} else {
		envName = "dev"
	}
	templateData := playbooks{
		GoTemplateRemoteName: templateInfo.RemoteName,
		FleetInfraEnv:        envName,
		UpdateNumber:         strconv.FormatUint(uint64(templateInfo.UpdateTransactionID), 10),
		RepoURL:              "https://{{ s3_buckets[fleet_infra_env] | default('rh-edge-tarballs-prod') }}.s3.us-east-1.amazonaws.com/{{ update_number }}/upd/{{ update_number }}/repo",
	}

	fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", account, templateInfo.UpdateTransactionID)
	tmpfilepath := fmt.Sprintf("/tmp/%s", fname)
	f, err := os.Create(tmpfilepath)
	if err != nil {
		s.log.WithField("error", err.Error()).Errorf("Error creating file")
		return "", err
	}
	err = templateContents.Execute(f, templateData)
	if err != nil {
		s.log.WithField("error", err.Error()).Errorf("Error executing template")
		return "", err
	}

	uploadPath := fmt.Sprintf("%s/playbooks/%s", account, fname)
	playbookURL, err := s.FilesService.GetUploader().UploadFile(tmpfilepath, uploadPath)
	if err != nil {
		s.log.WithField("error", err.Error()).Errorf("Error uploading file to S3")
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
	s.log.Infof("Update was finished")
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
		).Group("id").Order("id").Find(&updates)
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
	result := db.DB.Where(&models.DispatchRecord{PlaybookDispatcherID: e.Payload.ID}).First(&dispatchRecord)
	if result.Error != nil {
		return result.Error
	}

	if e.Payload.Status == PlaybookStatusFailure || e.Payload.Status == PlaybookStatusTimeout {
		dispatchRecord.Status = models.DispatchRecordStatusError
	} else if e.Payload.Status == PlaybookStatusSuccess {
		// TODO: We might wanna check if it's really success by checking the running hash on the device here
		dispatchRecord.Status = models.DispatchRecordStatusComplete
	} else if e.Payload.Status == PlaybookStatusRunning {
		dispatchRecord.Status = models.DispatchRecordStatusRunning
	} else {
		dispatchRecord.Status = models.DispatchRecordStatusError
		s.log.Fatal("Playbook status is not on the json schema for this event")
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
	result := db.DB.Preload("DispatchRecords").
		Table("update_transactions").
		Joins(
			`JOIN updatetransaction_dispatchrecords ON update_transactions.id = updatetransaction_dispatchrecords.update_transaction_id`).
		Where(`updatetransaction_dispatchrecords.dispatch_record_id = ?`,
			dispatchRecord.ID,
		).First(&update)
	if result.Error != nil {
		return result.Error
	}

	return s.SetUpdateStatus(&update)

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
	result := db.DB.Save(update)
	return result.Error
}
