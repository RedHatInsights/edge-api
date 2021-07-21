package playbooks

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/commits"
	log "github.com/sirupsen/logrus"
)

type playboppoks struct {
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

func WriteTemplate(tempalteInfo TemplateRemoteInfo) {

	filePath := "../template/"
	templateName := "template_playbook_dispatcher_ostree_upgrade_payload.yml"
	template, err := template.ParseFiles(filePath + templateName)
	if err != nil {
		fmt.Println(err)
		return
	}
	templateData := playboppoks{
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
	// f, err := os.Create("../template/playbook.yml")
	if err != nil {
		log.Println("create file: ", err)
		return
	}
	err = template.Execute(f, templateData)
	if err != nil {
		fmt.Println(err)
	}

	f.Close()
	uploadTemplate(tempalteInfo, "../template/")

}

func uploadTemplate(tempalteInfo TemplateRemoteInfo, tempalte_path string) {
	cfg := config.Get()
	path := filepath.Join(tempalte_path, "playbook.yml")
	var uploader commits.Uploader
	uploader = &commits.FileUploader{
		BaseDir: path,
	}
	if cfg.BucketName != "" {
		uploader = commits.NewS3Uploader()
	}
	log.Debug("::BuildUpdateRepo:uploader.UploadRepo: BEGIN")
	repoURL, err := uploader.UploadRepo(filepath.Join(path, "playbook"), "playbook.yml")
	log.Debug("::BuildUpdateRepo:uploader.UploadRepo: FINISH")
	fmt.Printf("::BuildUpdateRepo:repoURL: %v", repoURL)
	if err != nil {
		return
	}
}
