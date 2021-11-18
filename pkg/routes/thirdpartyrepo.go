package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

type tprepoTypeKey int

const tprepoKey tprepoTypeKey = iota

// MakeThirdPartyRepoRouter adds suport for operation on ThirdPartyRepo
func MakeThirdPartyRepoRouter(sub chi.Router) {
	sub.With(validateGetAllThirdPartyRepoFilterParams).With(common.Paginate).Get("/", GetAllThirdPartyRepo)
	sub.Post("/", CreateThirdPartyRepo)
	sub.Route("/{ID}", func(r chi.Router) {
		r.Use(ThirdPartyRepoCtx)
		r.Get("/", GetThirdPartyRepoByID)
		r.Put("/", CreateThirdPartyRepoUpdate)
		r.Delete("/", DeleteThirdPartyRepoByID)
	})
}

var thirdPartyRepoFilters = common.ComposeFilters(
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "third_party_repos.name",
	}),
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "third_party_repos.created_at",
	}),
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "updated_at",
		DBField:    "third_party_repos.updated_at",
	}),
	common.SortFilterHandler("third_party_repos", "created_at", "DESC"),
)

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
	result := thirdPartyRepoFilters(r, db.DB)
	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	pagination := common.GetPagination(r)
	countResult := thirdPartyRepoFilters(r, db.DB.Model(&models.ThirdPartyRepo{})).Where("account = ?", account).Count(&count)
	if countResult.Error != nil {
		countErr := errors.NewInternalServerError()
		log.Error(countErr)
		w.WriteHeader(countErr.GetStatus())
		json.NewEncoder(w).Encode(&countErr)
		return
	}
	log.Debugf("r.URL.Query() %v \n", r.URL.Query().Get("sort_by"))
	if r.URL.Query().Get("sort_by") != "name" && r.URL.Query().Get("sort_by") != "-name" {
		result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("account = ?", account).Find(&tprepo)
	}
	filter := r.URL.Query().Get("filter")
	filterMap := map[string]string{}
	if filter != "" {
		filterMap, err = validateFilterMap(filter)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
	}
	if err := result.Where(filterMap).Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo).Error; err != nil {
		err := errors.NewBadRequest("this is not a valid filter. filter must be in name.value")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	} else {
		result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("account = ?", account).Find(&tprepo)
	}
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
		if ID := chi.URLParam(r, "ID"); ID != "" {
			_, err := strconv.Atoi(ID)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
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

	s, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	ID := chi.URLParam(r, "ID")
	tprepo, err := s.ThirdPartyRepoService.GetThirdPartyRepoByID(ID)
	if err != nil {
		var responseErr errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryNotFound:
			responseErr = errors.NewNotFound(err.Error())
		default:
			responseErr = errors.NewInternalServerError()
			responseErr.SetTitle("failed getting third party repository")
		}
		log.Error(responseErr)
		w.WriteHeader(responseErr.GetStatus())
		json.NewEncoder(w).Encode(&responseErr)
		return
	}
	json.NewEncoder(w).Encode(&tprepo)
}

// CreateThirdPartyRepoUpdate updates the existing third party repository
func CreateThirdPartyRepoUpdate(w http.ResponseWriter, r *http.Request) {
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
	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}

	ID := chi.URLParam(r, "ID")
	err = services.ThirdPartyRepoService.UpdateThirdPartyRepo(tprepo, account, ID)
	if err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		err.SetTitle("failed updating third party repository")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	repoDetails, err := services.ThirdPartyRepoService.GetThirdPartyRepoByID(ID)
	if err != nil {
		log.Info(err)
	}
	json.NewEncoder(w).Encode(repoDetails)
}

// DeleteThirdPartyRepoByID deletes the third party repository using ID
func DeleteThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) {
	s, _ := r.Context().Value(dependencies.Key).(*dependencies.EdgeAPIServices)
	ID := chi.URLParam(r, "ID")

	tprepo, err := s.ThirdPartyRepoService.DeleteThirdPartyRepoByID(ID)
	if err != nil {
		var responseErr errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryNotFound:
			responseErr = errors.NewNotFound(err.Error())
		default:
			responseErr = errors.NewInternalServerError()
			responseErr.SetTitle("failed deleting third party repository")
		}
		log.Error(responseErr)
		w.WriteHeader(responseErr.GetStatus())
		json.NewEncoder(w).Encode(&responseErr)
		return
	}
	_ = tprepo
	json.NewEncoder(w).Encode(&tprepo)
}

func validateGetAllThirdPartyRepoFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs := []validationError{}
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()})
			}
		}
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" {
				name = val[1:]
			}
			if name != "name" && name != "created_at" && name != "updated_at" {
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be name or created_at or updated_at", name)})
			}
		}

		if len(errs) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&errs)
	})
}

func validateFilterMap(filter string) (map[string]string, error) {
	splits := strings.Split(filter, ".")
	if len(splits) != 2 {
		return nil, errors.NewBadRequest("this is not a valid filter. filter must be name")
	}
	field, value := splits[0], splits[1]
	return map[string]string{field: value}, nil

}
