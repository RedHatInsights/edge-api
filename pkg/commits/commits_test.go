package commits

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var cmt models.Commit

func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	tearDown()
	os.Exit(retCode)
}

func setUp() {
	config.Init()
	config.Get().Debug = true
	db.InitDB()
}

func tearDown() {
	db.DB.Delete(&cmt)
}

func mockCommit() {
	cmt.Account = "0000000"
	cmt.Name = "Test"
	db.DB.Create(&cmt)
}

type bodyResponse struct {
	Account string `json:"Account"`
}

func TestPatch(t *testing.T) {
	commitOne := &models.Commit{
		OSTreeRef: "one",
	}
	commitTwo := &models.Commit{
		OSTreeRef: "two",
	}

	applyPatch(commitOne, commitTwo)

	if commitOne.OSTreeRef != "two" {
		t.Errorf("expected two got %s", commitOne.OSTreeRef)
	}
}

func TestGetAllEmpty(t *testing.T) {
	err := db.DB.AutoMigrate(&models.Commit{})
	if err != nil {
		panic(err)
	}
	t.Run("returns empty commits", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetAll(response, request)
		got := []string{}
		json.NewDecoder(response.Body).Decode(&got)

		if len(got) != 0 {
			t.Errorf("got %q", got)
		}
	})

}

func TestGetAll(t *testing.T) {
	err := db.DB.AutoMigrate(models.Commit{})
	if err != nil {
		panic(err)
	}

	mockCommit()
	t.Run("returns Get all commits", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetAll(response, request)
		got := response.Body.String()
		if !strings.Contains(got, "0000000") {
			t.Errorf("got %q", got)
		}
	})
}

func TestGetById(t *testing.T) {
	err := db.DB.AutoMigrate(models.Commit{})
	if err != nil {
		panic(err)
	}

	mockCommit()
	t.Run("returns Get commit by id", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		GetByID(response, request)
		var bodyResp *bodyResponse
		json.NewDecoder(response.Body).Decode(&bodyResp)
		if bodyResp.Account != "0000000" {
			t.Errorf("got %q", bodyResp.Account)
		}
	})

}

func TestGetByIdFail(t *testing.T) {
	config.Init()
	config.Get().Debug = true

	db.InitDB()
	err := db.DB.AutoMigrate(models.Commit{})
	if err != nil {
		panic(err)
	}

	mockCommit()

	t.Run("returns Error", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetByID(response, request)
		got := response.Body.String()
		want := "must pass id\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestGetCommit(t *testing.T) {
	err := db.DB.AutoMigrate(models.Commit{})
	if err != nil {
		panic(err)
	}

	mockCommit()
	t.Run("returns Get commit ", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		getCommit(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("got %q", got)
		}
	})
}

func TestServeRepo(t *testing.T) {
	err := db.DB.AutoMigrate(models.Commit{})
	if err != nil {
		panic(err)
	}

	mockCommit()
	t.Run("returns Get commit ", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/repo", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)

		request = request.WithContext(ctx)

		ServeRepo(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("got %q", got)
		}
	})

}
