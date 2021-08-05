package playbooks

import (
	"fmt"
	"os"
	"text/template"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/files"
	log "github.com/sirupsen/logrus"
)

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

//S3Uploader defines the mechanism to upload data to S3
type S3Uploader struct {
	Client            *s3.S3
	S3ManagerUploader *s3manager.Uploader
	Bucket            string
}

// WriteTemplate will parse the values to the template
func WriteTemplate(templateInfo TemplateRemoteInfo) (string, error) {
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
	template.Execute(f, templateData)

	var uploader files.Uploader
	uploader = &files.FileUploader{
		BaseDir: fname,
	}
	if cfg.BucketName != "" {
		uploader = files.NewS3Uploader()
	}
	repoURL, err := uploader.UploadRepo(fname, fmt.Sprint(templateInfo.UpdateTransaction))
	if err != nil {
		log.Errorf("create file: %#v ", err)
		return "", err

	}
	log.Infof("create file:  %#v", repoURL)
	os.Remove(tmpfilepath)
	log.Infof("::WriteTemplate: ENDs")
	return repoURL, nil

}
