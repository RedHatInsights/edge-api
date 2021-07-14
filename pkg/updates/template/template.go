package main

import (
	"fmt"
	"log"
	"os"
	"text/template"
)

type playbook struct {
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

func main() {

	filePath := "../template/template_playbook_dispatcher_ostree_upgrade_payload.yml"
	template, err := template.ParseFiles(filePath)
	if err != nil {
		fmt.Println(err)
		return
	}
	templateData := playbook{
		GoTemplateRemoteName: "GO_TEMPLATE_REMOTE_NAME",
		GoTemplateRemoteURL:  "GO_TEMPLATE_REMOTE_URL",
		GoTemplateContentURL: "GO_TEMPLATE_CONTENT_URL",
		GoTemplateGpgVerify:  "GO_TEMPLATE_GPG_VERIFY",
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
