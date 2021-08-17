package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var cmt models.Commit

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
		t.Errorf("Expected two got %s", commitOne.OSTreeRef)
	}
}

func TestGetAllEmpty(t *testing.T) {

	t.Run("returns empty commits", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetAllCommits(response, request)
		got := []string{}
		fmt.Printf("Respose: %v\n", response.Body)
		json.NewDecoder(response.Body).Decode(&got)

		if len(got) != 0 {
			t.Errorf("Expected zero but got %q", got)
		}
	})

}

func TestGetAll(t *testing.T) {
	mockCommit()
	t.Run("returns Get all commits successfully", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetAllCommits(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})

}

func TestGetCommitById(t *testing.T) {
	mockCommit()
	t.Run("returns Get commit by id successfully", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		GetCommitByID(response, request)
		var bodyResp *bodyResponse
		json.NewDecoder(response.Body).Decode(&bodyResp)
		if bodyResp.Account != "0000000" {
			t.Errorf("Expected status code to be 0000000 but got %q", bodyResp.Account)
		}
	})

}

func TestGetByIdFail(t *testing.T) {
	mockCommit()

	t.Run("returns Error on get by id", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()

		GetCommitByID(response, request)
		got := response.Body.String()
		want := "must pass id\n"
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestGetCommit(t *testing.T) {
	mockCommit()
	t.Run("returns Get commit successfully", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		getCommit(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestServeRepo(t *testing.T) {
	mockCommit()
	t.Run("returns Serve Repo successfully", func(t *testing.T) {

		request, _ := http.NewRequest(http.MethodGet, "/repo", nil)
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)

		request = request.WithContext(ctx)

		ServeRepo(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})

}

func TestAdd(t *testing.T) {
	t.Run("returns Add Commit successfully", func(t *testing.T) {

		var jsonStr = []byte(`{ "Account": "123", "Name" :"test" }`)
		request, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBuffer(jsonStr))
		response := httptest.NewRecorder()

		AddCommit(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestAddError(t *testing.T) {
	t.Run("returns Error on add a commit", func(t *testing.T) {

		var jsonStr = []byte(`{bad json}`)
		request, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBuffer(jsonStr))
		response := httptest.NewRecorder()

		AddCommit(response, request)
		got := response.Code
		want := http.StatusBadRequest
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestUpdate(t *testing.T) {
	mockCommit()
	t.Run("returns update Commit successfully", func(t *testing.T) {
		var jsonStr = []byte(`{ "Account": "123"}`)

		request, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBuffer(jsonStr))
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		UpdateCommit(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestPatchF(t *testing.T) {
	mockCommit()
	t.Run("returns Patch ", func(t *testing.T) {
		var jsonStr = []byte(`{ "Account": "123"}`)

		request, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBuffer(jsonStr))
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		PatchCommit(response, request)
		got := response.Code
		want := http.StatusOK
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestPatchError(t *testing.T) {

	mockCommit()
	t.Run("returns Patch Error ", func(t *testing.T) {
		var jsonStr = []byte(`{bad json}`)

		request, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBuffer(jsonStr))
		response := httptest.NewRecorder()
		ctx := request.Context()
		ctx = context.WithValue(ctx, commitKey, &cmt)
		request = request.WithContext(ctx)
		PatchCommit(response, request)
		got := response.Code
		want := http.StatusBadRequest
		if got != want {
			t.Errorf("Expected status code to be %q but got %q", want, got)
		}
	})
}

func TestCommitCtx(t *testing.T) {
	mockCommit()

	t.Run("returns Get commitCtx ", func(t *testing.T) {
		next := http.HandlerFunc(final)
		got := CommitCtx(next)
		if got == nil {
			t.Errorf("Expected not nil response got %q", got)
		}

	})
}
func final(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}
