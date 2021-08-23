package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"text/template"

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
}

// NewUpdateService gives a instance of the main implementation of a UpdateServiceInterface
func NewUpdateService(ctx context.Context) UpdateServiceInterface {
	return &UpdateService{
		ctx:           ctx,
		deviceService: NewDeviceService(),
		repoBuilder:   NewRepoBuilder(ctx)}
}

// UpdateService is the main implementation of a UpdateServiceInterface
type UpdateService struct {
	ctx           context.Context
	repoBuilder   RepoBuilderInterface
	deviceService DeviceServiceInterface
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
	RemoteName        string
	RemoteURL         string
	ContentURL        string
	GpgVerify         string
	UpdateTransaction int
}

func (s *UpdateService) CreateUpdate(update *models.UpdateTransaction) (*models.UpdateTransaction, error) {
	update, err := s.repoBuilder.BuildUpdateRepo(update)
	if err != nil {
		// This is a goroutine and if this happens, the whole update failed
		log.Fatal(err)
	}
	// FIXME - implement playbook dispatcher scheduling
	// 1. Create template Playbook
	// 2. Upload templated playbook
	var remoteInfo TemplateRemoteInfo
	remoteInfo.RemoteURL = update.Repo.URL
	remoteInfo.RemoteName = "main-test"
	remoteInfo.ContentURL = update.Repo.URL
	remoteInfo.UpdateTransaction = int(update.ID)
	remoteInfo.GpgVerify = "true"
	playbookURL, err := s.writeTemplate(remoteInfo, update.Account)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	log.Debugf("playbooks:WriteTemplate: %#v", playbookURL)
	// 3. Loop through all devices in UpdateTransaction
	dispatchRecords := update.DispatchRecords
	for _, device := range update.Devices {
		var updateDevice *models.Device
		updateDevice, err = s.deviceService.GetDeviceByUUID(device.UUID)
		if err != nil {
			log.Errorf("Error on GetDeviceByUUID: %#v ", err.Error())
			return nil, err
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
					PlaybookURL:          update.Repo.URL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				device.Connected = false
				dispatchRecord := &models.DispatchRecord{
					Device:      updateDevice,
					PlaybookURL: update.Repo.URL,
					Status:      models.DispatchRecordStatusError,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			}

		}
		update.DispatchRecords = dispatchRecords
	}
	db.DB.Save(update)
	log.Infof("Repobuild::ends: update record %#v ", update)
	return update, nil
}

// WriteTemplate will parse the values to the template
func (s *UpdateService) writeTemplate(templateInfo TemplateRemoteInfo, account string) (string, error) {
	log.Infof("::WriteTemplate: BEGIN")
	cfg := config.Get()
	filePath := cfg.TemplatesPath
	templateName := "template_playbook_dispatcher_ostree_upgrade_payload.yml"
	template, err := template.ParseFiles(filePath + templateName)
	if err != nil {
		fmt.Println(err)
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
		OstreeGpgVerify:      "true",
		OstreeGpgKeypath:     "/etc/pki/rpm-gpg/",
		OstreeRemoteTemplate: "{{ ostree_remote_template }}"}

	fname := fmt.Sprintf("playbook_dispatcher_update_%v", templateInfo.UpdateTransaction) + ".yml"
	tmpfilepath := fmt.Sprintf("/tmp/%s", fname)
	f, err := os.Create(tmpfilepath)
	if err != nil {
		log.Errorf("create file: %#v", err)
		return "", err
	}
	err = template.Execute(f, templateData)
	if err != nil {
		log.Errorf("err: %#v ", err)
		return "", err
	}

	uploadPath := fmt.Sprintf("%s/playbooks/%s", account, fname)
	filesService := NewFilesService()
	repoURL, err := filesService.Uploader.UploadFile(tmpfilepath, uploadPath)
	if err != nil {
		log.Errorf("create file: %#v ", err)
		return "", err

	}
	log.Infof("create file:  %#v", repoURL)
	os.Remove(tmpfilepath)
	log.Infof("::WriteTemplate: ENDs")
	return repoURL, nil
}
