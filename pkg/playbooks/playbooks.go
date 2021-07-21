package playbooks

import (
	"fmt"
	"os"
	"text/template"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/commits"
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

type TemplateRemoteInfo struct {
	RemoteName        string
	RemoteURL         string
	ContentURL        string
	GpgVerify         string
	UpdateTransaction int
}

type S3Uploader struct {
	Client            *s3.S3
	S3ManagerUploader *s3manager.Uploader
	Bucket            string
}

func WriteTemplate(tempalteInfo TemplateRemoteInfo) (string, error) {
	log.Debugf("::WriteTemplate: BEGIN")
	filePath := "pkg/playbooks/"
	templateName := "template_playbook_dispatcher_ostree_upgrade_payload.yml"
	template, err := template.ParseFiles(filePath + templateName)
	if err != nil {
		fmt.Println(err)
		return "Playbook::WriteTemplate::Error::", err
	}
	templateData := playbooks{
		GoTemplateRemoteName: tempalteInfo.RemoteName,
		GoTemplateRemoteURL:  tempalteInfo.RemoteURL,
		GoTemplateContentURL: tempalteInfo.ContentURL,
		GoTemplateGpgVerify:  tempalteInfo.GpgVerify,
		OstreeRemoteName:     "{{ ostree_remote_name }}",
		OstreeRemoteURL:      "{{ ostree_remote_url }}",
		OstreeContentURL:     "{{ ostree_content_url }}",
		OstreeGpgVerify:      "true",
		OstreeGpgKeypath:     "/etc/pki/rpm-gpg/",
		OstreeRemoteTemplate: "{{ ostree_remote_template }}"}

	fname := fmt.Sprintf("playbook_dispatcher_update_%v", tempalteInfo.UpdateTransaction) + ".yml"
	path := filePath + fname
	f, err := os.Create(path)
	if err != nil {
		log.Println("create file: ", err)
		return "Playbook::WriteTemplate::Error::", err
	}
	template.Execute(f, templateData)

	cfg := config.Get()
	var uploader commits.Uploader
	uploader = &commits.FileUploader{
		BaseDir: path,
	}
	if cfg.BucketName != "" {
		uploader = commits.NewS3Uploader()
	}
	repoURL, err := uploader.UploadRepo(path, string(tempalteInfo.UpdateTransaction))
	if err != nil {
		log.Println("create file: ", err)
		return "Playbook::WriteTemplate::Error::", err

	}
	log.Println("create file: ", repoURL)
	os.Remove(path)
	log.Debugf("::WriteTemplate: ENDs")
	return repoURL, nil

}
