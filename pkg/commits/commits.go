package commits

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	log "github.com/sirupsen/logrus"

	"gorm.io/driver/sqlite"
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

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	log.Infof("Migrating database...")
	db.AutoMigrate(&Commit{})
}

func MakeRouter() chi.Router {

	cfg := config.Get()
	var sub chi.Router = chi.NewRouter()

	if cfg.Auth {
		sub.With(identity.EnforceIdentity).Get("/", common.StatusOK)
	} else {
		sub.Get("/", common.StatusOK)
	}

	sub.Post("/commits", Add)
	sub.Get("/commits", GetAll)
	sub.Route("/commits/{commitId}", func(r chi.Router) {
		r.Use(CommitCtx)
		r.Get("/", GetById)
	})

	return sub
}

func CommitCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var commit Commit
		if commitId := chi.URLParam(r, "commitId"); commitId != "" {
			id, err := strconv.Atoi(commitId)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			result := db.First(&commit, id)
			if result.Error != nil {
				http.Error(w, result.Error.Error(), http.StatusNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), "commit", &commit)
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

	db.Create(&commit)
}

func GetAll(w http.ResponseWriter, r *http.Request) {
	var commits []Commit
	result := db.Find(&commits)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(&commits)
}

func GetById(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	commit, ok := ctx.Value("commit").(*Commit)
	if !ok {
		http.Error(w, "must pass id", http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(commit)
}
