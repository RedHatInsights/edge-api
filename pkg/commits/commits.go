package commits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"gorm.io/gorm"
)

type Commit struct {
	gorm.Model
	Name          string
	Account       string
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
		account, err := getAccount(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if commitId := chi.URLParam(r, "commitId"); commitId != "" {
			id, err := strconv.Atoi(commitId)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			result := db.DB.Where("account = ?", account).First(&commit, id)
			if result.Error != nil {
				http.Error(w, result.Error.Error(), http.StatusNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), commitKey, &commit)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

func getAccount(r *http.Request) (string, error) {
	ident := identity.Get(r.Context())
	if ident.Identity.AccountNumber != "" {
		return ident.Identity.AccountNumber, nil
	}
	return "", fmt.Errorf("cannot find account number")

}

func Add(w http.ResponseWriter, r *http.Request) {

	var commit Commit
	var err error
	if err = json.NewDecoder(r.Body).Decode(&commit); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commit.Account, err = getAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.DB.Create(&commit)
}

func GetAll(w http.ResponseWriter, r *http.Request) {
	var commits []Commit
	account, err := getAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := db.DB.Where("account = ?", account).Find(&commits)
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
