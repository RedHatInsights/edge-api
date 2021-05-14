package commits

import (
	"encoding/json"
	"net/http"

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
	// sub.Get("/commits/{commitId}", GetById)

	return sub
}

// func CommitCtx(next http.Handler) http.Handler {
// 	var commit *Commit
// 	var err error

// 	if commitId := chi.URLParam(r, "commitId"); commitId != "" {
// 		commit = db.GET()
// 	}
// }

func Add(w http.ResponseWriter, r *http.Request) {

	var commit Commit
	if err := json.NewDecoder(r.Body).Decode(&commit); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("commit = %+v\n", commit)

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
