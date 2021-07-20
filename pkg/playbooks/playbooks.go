package playbooks

import (
	"fmt"
	"log"
	"os"
	"text/template"
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
	RemoteName string
	RemoteURL  string
	ContentURL string
	GpgVerify  string
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
	f, err := os.Create("../template/playbook.yml")
	if err != nil {
		log.Println("create file: ", err)
		return
	}
	err = template.Execute(f, templateData)
	if err != nil {
		fmt.Println(err)
	}

	f.Close()

}
