package commits

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"

	"github.com/redhatinsights/edge-api/pkg/db"
	"gorm.io/gorm"
)

type Commit struct {
	gorm.Model
	Name          string
	Hash          string
	BuildDate     string
	BuildNumber   uint
	BlueprintToml string
	NEVRAManifest string
	TarURL        string
}

func MakeRouter(sub chi.Router) {
	sub.Post("/", Add)
	sub.Get("/", GetAll)
	sub.Route("/{commitId}", func(r chi.Router) {
		r.Use(CommitCtx)
		r.Get("/", GetById)
	})
}

type key int

const commitKey key = 0

func CommitCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var commit Commit
		if commitId := chi.URLParam(r, "commitId"); commitId != "" {
			id, err := strconv.Atoi(commitId)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			result := db.DB.First(&commit, id)
			if result.Error != nil {
				http.Error(w, result.Error.Error(), http.StatusNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), commitKey, &commit)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

func Add(w http.ResponseWriter, r *http.Request) {

	var commit Commit
	if err := json.NewDecoder(r.Body).Decode(&commit); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// more code goes here to make sure we don't fuck up

	db.DB.Create(&commit)
}

func GetAll(w http.ResponseWriter, r *http.Request) {
	var commits []Commit
	result := db.DB.Find(&commits)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(&commits)
}

func GetById(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	commit, ok := ctx.Value(commitKey).(*Commit)
	if !ok {
		http.Error(w, "must pass id", http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(commit)
}
