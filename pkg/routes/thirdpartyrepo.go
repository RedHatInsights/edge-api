package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

type tprepoTypeKey int

const tprepoKey tprepoTypeKey = iota

// MakeTPRepoRouter adds suport for operation on ThirdPartyRepo
func MakeTPRepoRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", GetAllThirdPartyRepo)
	sub.Post("/", CreateThirdPartyRepo)
	sub.Route("/{tprepoId}", func(r chi.Router) {
		r.Use(ThirdPartyRepoCtx)
		r.Get("/", GetThirdPartyRepoByID)
	})
}

// A CreateTPRepoRequest model.
type CreateTPRepoRequest struct {
	Repo *models.ThirdPartyRepo
}

// CreateThirdPartyRepo creates Third Party Repository
func CreateThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	defer r.Body.Close()
	tprepo, err := createRequest(w, r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	log.Infof("ThirdPartyRepo::create: %#v", tprepo)

	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}

	err = services.ThirdPartyRepoService.CreateThirdPartyRepo(tprepo, account)

	if err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		err.SetTitle("failed creating third party repository")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&tprepo)

}

// createRequest validates request to create ThirdPartyRepo.
func createRequest(w http.ResponseWriter, r *http.Request) (*models.ThirdPartyRepo, error) {
	var tprepo *models.ThirdPartyRepo
	if err := json.NewDecoder(r.Body).Decode(&tprepo); err != nil {
		log.Error(err)
		err := errors.NewBadRequest("invalid JSON request")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil, err
	}

	if err := tprepo.ValidateRequest(); err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		return nil, err
	}

	return tprepo, nil
}

// GetAllThirdPartyRepo return all the ThirdPartyRepo
func GetAllThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	var tprepo *[]models.ThirdPartyRepo
	var count int64
	pagination := common.GetPagination(r)
	countResult := imageFilters(r, db.DB.Model(&models.ThirdPartyRepo{})).Count(&count)
	if countResult.Error != nil {
		countErr := errors.NewInternalServerError()
		log.Error(countErr)
		w.WriteHeader(countErr.GetStatus())
		json.NewEncoder(w).Encode(&countErr)
		return
	}
	result := db.DB.Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo)
	if result.Error != nil {
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"data": &tprepo, "count": count})

}

// ThirdPartyRepoCtx is a handler to Third Party Repository requests
func ThirdPartyRepoCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tprepo models.ThirdPartyRepo
		account, err := common.GetAccount(r)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		if tprepoId := chi.URLParam(r, "tprepoId"); tprepoId != "" {
			_, err := strconv.Atoi(tprepoId)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}
			result := db.DB.Where("account = ? and id = ?", account, tprepoId).Find(&tprepo)
			if result.Error != nil {
				err := errors.NewNotFound(result.Error.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}
			ctx := context.WithValue(r.Context(), tprepoKey, &tprepo)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// GetThirdPartyRepoByID gets the Third Party repository by ID from the database
func GetThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	tprepo, ok := ctx.Value(tprepoKey).(*models.ThirdPartyRepo)
	if !ok {
		err := errors.NewBadRequest("Must pass third party repository id")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}
	json.NewEncoder(w).Encode(&tprepo)
}
