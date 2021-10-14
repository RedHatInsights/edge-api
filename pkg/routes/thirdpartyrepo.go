package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// MakeTPRepoRouter adds suport for operation on ThirdyPartyRepo
func MakeTPRepoRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", ListAllThirdyPartyRepo)
	sub.Post("/", CreateThirdyPartyRepo)
}

// A CreateTPRepoRequest model.
type CreateTPRepoRequest struct {
	Repo *models.ThirdyPartyRepo
}

// CreateThirdyPartyRepo creates Third Party Repository
func CreateThirdyPartyRepo(w http.ResponseWriter, r *http.Request) {
	services, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	defer r.Body.Close()
	tprepo, err := initTPRepoCreateRequest(w, r)
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

	err = services.TPRepoService.CreateThirdyPartyRepo(tprepo, account)

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

// initTPRepoCreateRequest validates request to create thirdypartyrepo.
func initTPRepoCreateRequest(w http.ResponseWriter, r *http.Request) (*models.ThirdyPartyRepo, error) {
	var tprepo *models.ThirdyPartyRepo
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
	if tprepo.URL == "" {
		err := errors.NewBadRequest("URL is requird")
		w.WriteHeader(err.GetStatus())
		return nil, err
	}
	if tprepo.Name == "" {
		err := errors.NewBadRequest("Name is required")
		w.WriteHeader(err.GetStatus())
		return nil, err
	}

	return tprepo, nil
}

// ListAllThirdyPartyRepo return all the ThirdyPartyRepo
func ListAllThirdyPartyRepo(w http.ResponseWriter, r *http.Request) {
	var tprepo *[]models.ThirdyPartyRepo
	pagination := common.GetPagination(r)

	result := db.DB.Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo)
	if result.Error != nil {
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}

	json.NewEncoder(w).Encode(&tprepo)

}
