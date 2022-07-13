package common

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var dbName string

func setUp() {
	orgID := faker.UUIDHyphenated()
	config.Init()
	config.Get().Debug = true
	time := time.Now().UnixNano()
	dbName = fmt.Sprintf("%d-routes-common.db", time)
	config.Get().Database.Name = dbName
	db.InitDB()
	db.DB.AutoMigrate(&models.Image{})
	images := []models.Image{
		{
			Name:         "Motion Sensor 1",
			Distribution: "rhel-8",
			Status:       models.ImageStatusError,
			Commit:       &models.Commit{Arch: "arm7", OrgID: orgID},
			OrgID:        orgID,
		},
		{
			Name:         "Pressure Sensor 1",
			Distribution: "fedora-33",
			Status:       models.ImageStatusSuccess,
			Commit:       &models.Commit{Arch: "x86_64", OrgID: orgID},
			OrgID:        orgID,
		},
		{
			Name:         "Pressure Sensor 2",
			Distribution: "rhel-8",
			Status:       models.ImageStatusCreated,
			Commit:       &models.Commit{Arch: "x86_64", OrgID: orgID},
			OrgID:        orgID,
		},
		{
			Name:         "Motion Sensor 2",
			Distribution: "rhel-8",
			Status:       models.ImageStatusBuilding,
			Commit:       &models.Commit{Arch: "arm7", OrgID: orgID},
			OrgID:        orgID,
		},
	}
	db.DB.Create(&images)
}

func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	db.DB.Exec("DELETE FROM images")
	os.Remove(dbName)
	os.Exit(retCode)
}

func TestContainFilterHandler(t *testing.T) {
	filter := ComposeFilters(ContainFilterHandler(&Filter{
		QueryParam: "name",
		DBField:    "images.name",
	}))
	req, err := http.NewRequest(http.MethodGet, "/images?name=Motion", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %s", err)
	}
	result := filter(req, db.DB)
	images := []models.Image{}
	result.Find(&images)
	for _, image := range images {
		if !strings.Contains(image.Name, "Motion") {
			t.Errorf("Expected image will have Motion in it but got %s", image.Name)
		}
	}
}

func TestContainFilterHandlerWithMultiple(t *testing.T) {
	testStatusOne := "SUCCESS"
	testStatusTwo := "ERROR"
	filter := ComposeFilters(ContainFilterHandler(&Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}))
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/images?status=%sstatus=%s", testStatusOne, testStatusTwo), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %s", err)
	}
	result := filter(req, db.DB)
	images := []models.Image{}
	result.Find(&images)
	hasBothStatus := 0
	for _, image := range images {
		if image.Status == testStatusOne {
			hasBothStatus++
		} else if image.Status == testStatusTwo {
			hasBothStatus++
		} else {
			t.Errorf("Expected image status to be %s or %s but got %s", testStatusOne, testStatusTwo, image.Status)
		}
	}
	if hasBothStatus != len(images) {
		t.Errorf("Expected images with both status %s and %s to be returned but got only one status", testStatusOne, testStatusTwo)
	}
}

func TestOneOfFilterHandler(t *testing.T) {
	filter := ComposeFilters(OneOfFilterHandler(&Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}))
	req, err := http.NewRequest(http.MethodGet, "/images?status=CREATED&status=BUILDING", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %s", err)
	}
	result := filter(req, db.DB)
	images := []models.Image{}
	result.Find(&images)
	created := false
	building := false
	for _, image := range images {
		if image.Status == "CREATED" {
			created = true
		}
		if image.Status == "BUILDING" {
			building = true
		}
		if image.Status != "CREATED" && image.Status != "BUILDING" {
			t.Errorf("Expected image status will be CREATED or BUILDING but got %s", image.Status)
		}
	}
	if !building || !created {
		t.Errorf("Expected to see both BUILDING and CREATED but BUILDING %t and CREATED %t", building, created)
	}
}

func TestCreatedAtFilterHandler(t *testing.T) {
	filter := ComposeFilters(CreatedAtFilterHandler(&Filter{
		QueryParam: "created_at",
		DBField:    "images.created_at",
	}))
	nowStr := time.Now().Format(LayoutISO)
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/images?created_at=%s", nowStr), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %s", err)
	}
	result := filter(req, db.DB)
	images := []models.Image{}
	result.Find(&images)
	if len(images) == 0 {
		t.Fatalf("No images were found with created_at value: %s", nowStr)
	}
	for _, image := range images {
		if image.CreatedAt.Time.Format(LayoutISO) != nowStr {
			t.Errorf("Expected image created at will be %s but %s", nowStr, image.CreatedAt.Time.Format(LayoutISO))
		}
	}
}

func TestSortFilterHandler(t *testing.T) {
	filter := ComposeFilters(SortFilterHandler("images", "id", "ASC"), ContainFilterHandler(&Filter{
		QueryParam: "name",
		DBField:    "images.name",
	}))
	tt := []struct {
		url string
		asc bool
	}{
		{url: "/images?name=Pressure&sort_by=-name", asc: false},
		{url: "/images?name=Pressure&sort_by=name", asc: true},
	}

	for _, te := range tt {
		req, err := http.NewRequest(http.MethodGet, te.url, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %s", err)
		}
		result := filter(req, db.DB)
		images := []models.Image{}
		result.Find(&images)
		if !te.asc && images[0].Name < images[1].Name {
			t.Errorf("Expected first result name %s will be higher than second result %s", images[0].Name, images[1].Name)
		}
		if te.asc && images[0].Name > images[1].Name {
			t.Errorf("Expected first result name %s will be lower than second result %s", images[0].Name, images[1].Name)
		}
	}

}
