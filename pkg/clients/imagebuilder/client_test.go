package imagebuilder

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func setUp() {
	config.Init()
	config.Get().Debug = true
}

func tearDown() {

}

func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	tearDown()
	os.Exit(retCode)
}
func TestInitClient(t *testing.T) {
	ctx := context.Background()
	client := InitClient(ctx, &log.Entry{})
	if client == nil {
		t.Errorf("Client shouldnt be nil")
	}
}

func TestComposeImage(t *testing.T) {
	config.Init()

	ctx := context.Background()
	client := InitClient(ctx, log.NewEntry(log.StandardLogger()))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, `{"id": "compose-job-id-returned-from-image-builder"}`)
	}))
	defer ts.Close()
	config.Get().ImageBuilderConfig.URL = ts.URL

	pkgs := []models.Package{
		{
			Name: "vim",
		},
		{
			Name: "ansible",
		},
	}
	img := &models.Image{Distribution: "rhel-8",
		Packages: pkgs,
		Commit: &models.Commit{
			Arch: "x86_64",
		}}
	img, err := client.ComposeCommit(img)
	if err != nil {
		t.Errorf("Shouldnt throw error")
	}
	if img == nil {
		t.Errorf("Image shouldnt be nil")
	}
	if img != nil && img.Commit.ComposeJobID != "compose-job-id-returned-from-image-builder" {
		t.Error("Compose job is not correct")
	}
}
