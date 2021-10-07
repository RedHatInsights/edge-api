package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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
	CreateUpdate(update *models.UpdateTransaction) (*models.UpdateTransaction, error)
	GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error)
	RebootDevice(update *models.UpdateTransaction)
	GetUpdateTransactionsForDevice(device *models.Device) (*[]models.UpdateTransaction, error)
}

// NewUpdateService gives a instance of the main implementation of a UpdateServiceInterface
func NewUpdateService(ctx context.Context) UpdateServiceInterface {
	return &UpdateService{
		ctx:          ctx,
		filesService: NewFilesService(),
		repoBuilder:  NewRepoBuilder(ctx),
	}
}

const DelayTimeToReboot = 5

// UpdateService is the main implementation of a UpdateServiceInterface
type UpdateService struct {
	ctx          context.Context
	repoBuilder  RepoBuilderInterface
	filesService FilesService
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

func (s *UpdateService) CreateUpdate(update *models.UpdateTransaction) (*models.UpdateTransaction, error) {
	update.Status = models.UpdateStatusBuilding
	db.DB.Save(&update)
	update, err := s.repoBuilder.BuildUpdateRepo(update)
	if err != nil {
		update.Status = models.UpdateStatusError
		db.DB.Save(update)
		// This is a goroutine and if this happens, the whole update failed
		log.Fatal(err)
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
		client := playbookdispatcher.InitClient(s.ctx)
		exc, err := client.ExecuteDispatcher(payloadDispatcher)

		if err != nil {
			log.Errorf("Error on playbook-dispatcher-executuin: %#v ", err)
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
	}
	// TODO: This has to change to be after the reboot if new ostree commit == desired commit
	update.Status = models.UpdateStatusSuccess
	db.DB.Save(update)
	log.Infof("Repobuild::ends: update record %#v ", update)

	s.RebootDevice(update)

	log.Infof("Update was finished for :: %d", update.ID)
	return update, nil
}

func (s *UpdateService) GetUpdatePlaybook(update *models.UpdateTransaction) (io.ReadCloser, error) {
	fname := fmt.Sprintf("playbook_dispatcher_update_%d.yml", update.ID)
	path := fmt.Sprintf("%s/playbooks/%s", update.Account, fname)
	return s.filesService.GetFile(path)
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
	playbookURL, err := s.filesService.GetUploader().UploadFile(tmpfilepath, uploadPath)
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

func (s *UpdateService) RebootDevice(update *models.UpdateTransaction) {
	log.Infof("Execute Reboot Device for update ID :: %d", update.ID)
	timer := time.AfterFunc(time.Minute*DelayTimeToReboot, func() {
		log.Infof("Waiting time over to apply update :: %d", update.ID)
		dispatchRecords := update.DispatchRecords

		cfg := config.Get()
		filePath := cfg.TemplatesPath
		templateName := "template_playbook_dispatcher_reboot_device.yml"
		template, err := template.ParseFiles(filePath + templateName)
		fmt.Printf("template: %v", template)
		if err != nil {
			log.Errorf("Error parsing playbook template  :: %s", err.Error())
		}
		fname := fmt.Sprintf("playbook_dispatcher_reboot_%d.yml", update.ID)
		tmpfilepath := fmt.Sprintf("/tmp/%s", fname)
		if err != nil {
			log.Errorf("Error creating file: %s", err.Error())

		}

		uploadPath := fmt.Sprintf("%s/playbooks/%s", update.Account, fname)

		playbookURL, err := s.filesService.GetUploader().UploadFile(tmpfilepath, uploadPath)
		if err != nil {
			log.Errorf("Error uploading thet  file: %s", err.Error())

		}
		// Create new &DispatcherPayload{}
		payloadDispatcher := playbookdispatcher.DispatcherPayload{
			Recipient:   update.Devices[len(update.Devices)-1].RHCClientID,
			PlaybookURL: playbookURL,
			Account:     update.Account,
		}

		log.Infof("Call Execute Dispatcher Reboot: : %#v", payloadDispatcher)
		client := playbookdispatcher.InitClient(s.ctx)
		exc, err := client.ExecuteDispatcher(payloadDispatcher)
		if err != nil {
			log.Errorf("Error on playbook-dispatcher-executuin: %#v ", err)

		}
		for _, excPlaybook := range exc {
			if excPlaybook.StatusCode == http.StatusCreated {
				update.Devices[len(update.Devices)-1].Connected = true
				dispatchRecord := &models.DispatchRecord{
					Device:               &update.Devices[len(update.Devices)-1],
					PlaybookURL:          playbookURL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				update.Devices[len(update.Devices)-1].Connected = false
				dispatchRecord := &models.DispatchRecord{
					Device:      &update.Devices[len(update.Devices)-1],
					PlaybookURL: playbookURL,
					Status:      models.DispatchRecordStatusError,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			}
			update.DispatchRecords = dispatchRecords
		}
		db.DB.Save(update)
		log.Infof("Reboot playbook applied :: %d", update.ID)
	})

	<-timer.C
}

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
