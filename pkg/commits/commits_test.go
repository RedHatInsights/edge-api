package commits

import (
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
			t.Errorf("got %q", got)
		}
	})
	db.DB.Delete(&cmt)

}
