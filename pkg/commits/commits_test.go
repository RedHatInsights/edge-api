package commits

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
)

func TestPatch(t *testing.T) {
	commitOne := &Commit{
		OSTreeRef: "one",
	}
	commitTwo := &Commit{
		OSTreeRef: "two",
	}

	applyPatch(commitOne, commitTwo)

	if commitOne.OSTreeRef != "two" {
		t.Errorf("expected two got %s", commitOne.OSTreeRef)
	}
}

func TestGetAllEmpty(t *testing.T) {
	config.Init()
	config.Get().Debug = true
	db.InitDB()
	err := db.DB.AutoMigrate(Commit{})
	if err != nil {
		panic(err)
	}
	t.Run("returns empty commits", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetAll(response, request)

		got := response.Body.String()
		want := "[]\n"

		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

}

func TestGetAll(t *testing.T) {
	config.Init()
	config.Get().Debug = true

	db.InitDB()
	err := db.DB.AutoMigrate(Commit{})
	if err != nil {
		panic(err)
	}

	var cmt Commit
	cmt.Account = "0000000"
	cmt.Name = "Test"
	db.DB.Create(&cmt)

	t.Run("returns Get all commits", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetAll(response, request)
		got := response.Body.String()
		if !strings.Contains(got, "0000000") {
			db.DB.Delete(&cmt)
			t.Errorf("got %q", got)
		}
	})
	db.DB.Delete(&cmt)
}

func TestGetById(t *testing.T) {
	config.Init()
	config.Get().Debug = true

	db.InitDB()
	err := db.DB.AutoMigrate(Commit{})
	if err != nil {
		panic(err)
	}

	var cmt Commit
	cmt.Account = "0000000"
	cmt.Name = "Test"
	db.DB.Create(&cmt)

	t.Run("returns Get commit by id", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		GetByID(response, request)
		got := response.Body.String()
		if !strings.Contains(got, "0000000") {
			db.DB.Delete(&cmt)
			t.Errorf("got %q", got)
		}
	})
	db.DB.Delete(&cmt)
}

func TestGetByIdFail(t *testing.T) {
	config.Init()
	config.Get().Debug = true

	db.InitDB()
	err := db.DB.AutoMigrate(Commit{})
	if err != nil {
		panic(err)
	}

	var cmt Commit
	cmt.Account = "0000000"
	cmt.Name = "Test"
	db.DB.Create(&cmt)

	t.Run("returns Error", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetByID(response, request)
		got := response.Body.String()
		want := "must pass id\n"
		if got != want {
			db.DB.Delete(&cmt)
			t.Errorf("got %q, want %q", got, want)
		}
	})
	db.DB.Delete(&cmt)
}

func TestGetCommit(t *testing.T) {
	config.Init()
	config.Get().Debug = true

	db.InitDB()
	err := db.DB.AutoMigrate(Commit{})
	if err != nil {
		panic(err)
	}

	var cmt Commit
	cmt.Account = "0000000"
	cmt.Name = "Test"
	db.DB.Create(&cmt)

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
			db.DB.Delete(&cmt)
			t.Errorf("got %q", got)
		}
	})
	db.DB.Delete(&cmt)
}

func TestServeRepo(t *testing.T) {
	config.Init()
	config.Get().Debug = true

	db.InitDB()
	err := db.DB.AutoMigrate(Commit{})
	if err != nil {
		panic(err)
	}

	var cmt Commit
	cmt.Account = "0000000"
	cmt.Name = "Test"
	db.DB.Create(&cmt)

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
			db.DB.Delete(&cmt)
			t.Errorf("got %q", got)
		}
	})
	db.DB.Delete(&cmt)
}
