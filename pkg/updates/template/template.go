package main

import (
	"fmt"
	"log"
	"os"
	"text/template"
)

type playbook struct {
	GO_TEMPLATE_REMOTE_NAME string
	GO_TEMPLATE_REMOTE_URL  string
	GO_TEMPLATE_CONTENT_URL string
	GO_TEMPLATE_GPG_VERIFY  string
	Ostree_remote_name      string
	Ostree_remote_url       string
	Ostree_content_url      string
	Ostree_gpg_verify       string
	Ostree_gpg_keypath      string
	Ostree_remote_template  string
}

func main() {

	filePath := "../template/template_playbook_dispatcher_ostree_upgrade_payload.yml"
	template, err := template.ParseFiles(filePath)
	if err != nil {
		fmt.Println(err)
		return
	}
	template_data := playbook{
		GO_TEMPLATE_REMOTE_NAME: "GO_TEMPLATE_REMOTE_NAME",
		GO_TEMPLATE_REMOTE_URL:  "GO_TEMPLATE_REMOTE_URL",
		GO_TEMPLATE_CONTENT_URL: "GO_TEMPLATE_CONTENT_URL",
		GO_TEMPLATE_GPG_VERIFY:  "GO_TEMPLATE_GPG_VERIFY",
		Ostree_remote_name:      "{{ ostree_remote_name }}",
		Ostree_remote_url:       "{{ ostree_remote_url }}",
		Ostree_content_url:      "{{ ostree_content_url }}",
		Ostree_gpg_verify:       "true",
		Ostree_gpg_keypath:      "/etc/pki/rpm-gpg/",
		Ostree_remote_template:  "{{ ostree_remote_template }}"}
	f, err := os.Create("../template/playbook.yml")
	if err != nil {
		log.Println("create file: ", err)
		return
	}
	err = template.Execute(f, template_data)
	if err != nil {
		fmt.Println(err)
	}

	f.Close()

}
