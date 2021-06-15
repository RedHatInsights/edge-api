package imagebuilder

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func TestInitClient(t *testing.T) {
	InitClient()
	if Client == nil {
		t.Errorf("Client shouldnt be nil")
	}
}

func TestComposeImage(t *testing.T) {
	config.Init()

	InitClient()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"id": "compose-job-id-returned-from-image-builder"}`)
	}))
	defer ts.Close()
	config.Get().ImageBuilderConfig.URL = ts.URL

	pkgs := []models.Package{
		models.Package{
			Name: "vim",
		},
		models.Package{
			Name: "ansible",
		},
	}
	img := &models.Image{Distribution: "rhel-8", OutputType: "tar", Commit: &models.Commit{
		Arch:     "x86_64",
		Packages: pkgs,
	}}
	img, err := Client.Compose(img)
	if err != nil {
		t.Errorf("Shouldnt throw error")
	}
	if img == nil {
		t.Errorf("Image shouldnt be nil")
	}
	if img != nil && img.ComposeJobID != "compose-job-id-returned-from-image-builder" {
		t.Error("Compose job is not correct")
	}
}
