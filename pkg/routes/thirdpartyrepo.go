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
		r.Put("/", UpdateThirdPartyRepo)
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

func getThirdPartyRepo(w http.ResponseWriter, r *http.Request) *models.ThirdPartyRepo {
	ctx := r.Context()
	tprepo, ok := ctx.Value(tprepoKey).(*models.ThirdPartyRepo)
	if !ok {
		err := errors.NewBadRequest("Failed getting third party repo from context")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil
	}
	return tprepo
}

// CreateThirdPartyRepo creates Third Party Repository
func CreateThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	defer r.Body.Close()
	thirdPartyRepo, err := createRequest(w, r)
	if err != nil {
		// error handled by createRequest already
		return
	}
	services.Log.Info("Creating third party repository")

	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Account was not set")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}

	thirdPartyRepo, err = services.ThirdPartyRepoService.CreateThirdPartyRepo(thirdPartyRepo, account)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error creating third party repository")
		err := errors.NewInternalServerError()
		err.SetTitle("failed creating third party repository")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&thirdPartyRepo)

}

// createRequest validates request to create ThirdPartyRepo.
func createRequest(w http.ResponseWriter, r *http.Request) (*models.ThirdPartyRepo, error) {
	services := dependencies.ServicesFromContext(r.Context())

	var tprepo *models.ThirdPartyRepo
	if err := json.NewDecoder(r.Body).Decode(&tprepo); err != nil {
		services.Log.WithField("error", err.Error()).Error("Error parsing json from third party repo")
		err := errors.NewBadRequest("invalid JSON request")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return nil, err
	}
	services.Log = services.Log.WithFields(log.Fields{
		"URL":  tprepo.URL,
		"name": tprepo.Name,
	})

	if err := tprepo.ValidateRequest(); err != nil {
		services.Log.WithField("error", err.Error()).Info("Error validation request from third party repo")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		return nil, err
	}

	return tprepo, nil
}

// GetAllThirdPartyRepo return all the ThirdPartyRepo
func GetAllThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	services := dependencies.ServicesFromContext(r.Context())
	var tprepo *[]models.ThirdPartyRepo
	var count int64
	result := thirdPartyRepoFilters(r, db.DB)
	account, err := common.GetAccount(r)
	if err != nil {
		services.Log.WithField("error", err.Error()).Error("Error retrieving account from the request")
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}
	pagination := common.GetPagination(r)
	countResult := thirdPartyRepoFilters(r, db.DB.Model(&models.ThirdPartyRepo{})).Where("account = ?", account).Count(&count)
	if countResult.Error != nil {
		services.Log.WithField("error", err.Error()).Error("Error counting results")
		countErr := errors.NewInternalServerError()
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
		services.Log.WithField("error", err.Error()).Debug("Error parsing pagination filters")
		err := errors.NewBadRequest("this is not a valid filter. filter must be in name.value")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
		return
	}

	result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("account = ?", account).Find(&tprepo)
	if result.Error != nil {
		services.Log.WithField("error", err.Error()).Error("Error returning results")
		err := errors.NewBadRequest("Not Found")
		w.WriteHeader(err.GetStatus())
		json.NewEncoder(w).Encode(&err)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"data": &tprepo, "count": count})

}

// ThirdPartyRepoCtx is a handler to Third Party Repository requests
func ThirdPartyRepoCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := dependencies.ServicesFromContext(r.Context())
		if ID := chi.URLParam(r, "ID"); ID != "" {
			_, err := strconv.Atoi(ID)
			s.Log = s.Log.WithField("thirdPartyRepoID", ID)
			s.Log.Debug("Retrieving third party repo")
			if err != nil {
				s.Log.Debug("ID is not an integer")
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}

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
				w.WriteHeader(responseErr.GetStatus())
				json.NewEncoder(w).Encode(&responseErr)
				return
			}
			account, err := common.GetAccount(r)
			if err != nil || tprepo.Account != account {
				log.WithFields(log.Fields{
					"error":   err.Error(),
					"account": account,
				}).Error("Error retrieving account or third party repo doesn't belong to account")
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.GetStatus())
				json.NewEncoder(w).Encode(&err)
				return
			}
			ctx := context.WithValue(r.Context(), tprepoKey, tprepo)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			s.Log.Debug("Third Party Repo ID was not passed to the request or it was empty")
			err := errors.NewBadRequest("Third Party Repo ID required")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
	})
}

// GetThirdPartyRepoByID gets the Third Party repository by ID from the database
func GetThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) {
	if tprepo := getThirdPartyRepo(w, r); tprepo != nil {
		json.NewEncoder(w).Encode(tprepo)
	}
}

// UpdateThirdPartyRepo updates the existing third party repository
func UpdateThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	if oldtprepo := getThirdPartyRepo(w, r); oldtprepo != nil {
		services := dependencies.ServicesFromContext(r.Context())
		defer r.Body.Close()
		tprepo, err := createRequest(w, r)
		if err != nil {
			// error handled by createRequest already
			return
		}
		err = services.ThirdPartyRepoService.UpdateThirdPartyRepo(tprepo, oldtprepo.Account, fmt.Sprint(oldtprepo.ID))
		if err != nil {
			services.Log.WithField("error", err.Error()).Error("Error updating third party repository")
			err := errors.NewInternalServerError()
			err.SetTitle("failed updating third party repository")
			w.WriteHeader(err.GetStatus())
			json.NewEncoder(w).Encode(&err)
			return
		}
		w.WriteHeader(http.StatusOK)
		repoDetails, err := services.ThirdPartyRepoService.GetThirdPartyRepoByID(fmt.Sprint(oldtprepo.ID))
		if err != nil {
			log.Info(err)
		}
		json.NewEncoder(w).Encode(repoDetails)
	}
}

// DeleteThirdPartyRepoByID deletes the third party repository using ID
func DeleteThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) {
	if tprepo := getThirdPartyRepo(w, r); tprepo != nil {
		s := dependencies.ServicesFromContext(r.Context())

		tprepo, err := s.ThirdPartyRepoService.DeleteThirdPartyRepoByID(fmt.Sprint(tprepo.ID))
		if err != nil {
			var responseErr errors.APIError
			switch err.(type) {
			case *services.ThirdPartyRepositoryNotFound:
				responseErr = errors.NewNotFound(err.Error())
			default:
				responseErr = errors.NewInternalServerError()
				responseErr.SetTitle("failed deleting third party repository")
			}
			s.Log.WithField("error", err.Error()).Error("Error deleting third party repository")
			w.WriteHeader(responseErr.GetStatus())
			json.NewEncoder(w).Encode(&responseErr)
			return
		}
		_ = tprepo
		json.NewEncoder(w).Encode(&tprepo)
	}
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
