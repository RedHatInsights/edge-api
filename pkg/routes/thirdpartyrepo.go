// FIXME: golangci-lint
// nolint:gocritic,revive
package routes

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"gorm.io/gorm"
)

type tprepoTypeKey int

const tprepoKey tprepoTypeKey = iota

// MakeThirdPartyRepoRouter adds support for operation on ThirdPartyRepo
func MakeThirdPartyRepoRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("thirdpartyrepo")).With(validateGetAllThirdPartyRepoFilterParams).With(common.Paginate).Get("/", GetAllThirdPartyRepo)
	sub.Post("/", CreateThirdPartyRepo)
	sub.Route("/checkName", func(r chi.Router) {
		r.Get("/{name}", CheckThirdPartyRepoName)
	})
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

func CheckThirdPartyRepoName(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	name := chi.URLParam(r, "name")
	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	value, err := ctxServices.ThirdPartyRepoService.ThirdPartyRepoNameExists(orgID, name)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.OrgIDNotSet, *services.ThirdPartyRepositoryNameIsEmpty:
			apiError = errors.NewBadRequest(err.Error())
		default:
			apiError = errors.NewInternalServerError()
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": map[string]interface{}{"isValid": value}})
}

// A CreateTPRepoRequest model.
type CreateTPRepoRequest struct {
	Repo *models.ThirdPartyRepo
}

func getThirdPartyRepo(w http.ResponseWriter, r *http.Request) *models.ThirdPartyRepo {
	ctx := r.Context()
	ctxServices := dependencies.ServicesFromContext(ctx)
	tprepo, ok := ctx.Value(tprepoKey).(*models.ThirdPartyRepo)
	if !ok {
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting custom repository from context"))
		return nil
	}
	return tprepo
}

// CreateThirdPartyRepo creates Third Party Repository
func CreateThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	thirdPartyRepo, err := createRequest(w, r)
	if err != nil {
		// error handled by createRequest already
		return
	}
	ctxServices.Log.Info("Creating custom repository")

	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}

	thirdPartyRepo, err = ctxServices.ThirdPartyRepoService.CreateThirdPartyRepo(thirdPartyRepo, orgID)
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryNameIsEmpty, *services.ThirdPartyRepositoryURLIsEmpty, *services.ThirdPartyRepositoryAlreadyExists:
			apiError = errors.NewBadRequest(err.Error())
		default:
			apiError = errors.NewInternalServerError()
			apiError.SetTitle("failed creating custom repository")
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, &thirdPartyRepo)
}

// createRequest validates request to create ThirdPartyRepo.
func createRequest(w http.ResponseWriter, r *http.Request) (*models.ThirdPartyRepo, error) {
	ctxServices := dependencies.ServicesFromContext(r.Context())

	var tprepo *models.ThirdPartyRepo
	if err := readRequestJSONBody(w, r, ctxServices.Log, &tprepo); err != nil {
		return nil, err
	}

	if err := tprepo.ValidateRequest(); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Info("custom repository validation error")
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
		return nil, err
	}
	return tprepo, nil
}

// GetAllThirdPartyRepo return all the ThirdPartyRepo
func GetAllThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	ctxServices := dependencies.ServicesFromContext(r.Context())
	var tprepo []models.ThirdPartyRepo
	var count int64

	orgID := readOrgID(w, r, ctxServices.Log)
	if orgID == "" {
		// logs and response handled by readOrgID
		return
	}
	var ctx *gorm.DB
	imageID := r.URL.Query().Get("imageID")
	if imageID != "" {
		ctx = db.Org(orgID, "").Debug().
			Joins("left join images_repos on third_party_repo_id = id and image_id = ?", imageID).
			Order("images_repos.third_party_repo_id DESC NULLS LAST").
			Model(&models.ThirdPartyRepo{})
		ctx = thirdPartyRepoFilters(r, ctx)
	} else {
		ctx = db.OrgDB(orgID, thirdPartyRepoFilters(r, db.DB), "").Debug().Model(&models.ThirdPartyRepo{})
	}

	// Check to see if feature is enabled and not in ephemeral
	//cfg := config.Get()
	//if cfg.FeatureFlagsEnvironment != "ephemeral" && cfg.FeatureFlagsURL != "" {
	//	enabled := feature.CheckFeature(feature.FeatureCustomRepos)
	//	if !enabled {
	//		respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable("Feature not available"))
	//		return
	//	}
	//}

	pagination := common.GetPagination(r)

	if result := ctx.Count(&count); result.Error != nil {
		ctxServices.Log.WithField("error", result.Error).Error("Error counting results")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	if imageID != "" {
		if result := ctx.Preload("Images", "id = ?", imageID).Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo); result.Error != nil {
			ctxServices.Log.WithField("error", result.Error).Error("Error returning results")
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}
	} else {
		if result := ctx.Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo); result.Error != nil {
			ctxServices.Log.WithField("error", result.Error).Error("Error returning results")
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
			return
		}
	}

	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": &tprepo, "count": count})
}

// ThirdPartyRepoCtx is a handler to Third Party Repository requests
func ThirdPartyRepoCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		if ID := chi.URLParam(r, "ID"); ID != "" {
			_, err := strconv.Atoi(ID)
			ctxServices.Log = ctxServices.Log.WithField("thirdPartyRepoID", ID)
			ctxServices.Log.Debug("Retrieving custom repository")
			if err != nil {
				ctxServices.Log.Debug("ID is not an integer")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))
				return
			}

			tprepo, err := ctxServices.ThirdPartyRepoService.GetThirdPartyRepoByID(ID)
			if err != nil {
				var responseErr errors.APIError
				switch err.(type) {
				case *services.ThirdPartyRepositoryNotFound:
					responseErr = errors.NewNotFound(err.Error())
				default:
					responseErr = errors.NewInternalServerError()
					responseErr.SetTitle("failed getting custom repository")
				}
				respondWithAPIError(w, ctxServices.Log, responseErr)
				return
			}
			orgID := readOrgID(w, r, ctxServices.Log)
			if orgID == "" {
				// logs and response handled by readOrgID
				return
			}
			ctx := context.WithValue(r.Context(), tprepoKey, tprepo)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			ctxServices.Log.Debug("custom repository ID was not passed to the request or it was empty")
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Custom repository ID is required"))
			return
		}
	})
}

// GetThirdPartyRepoByID gets the Third Party repository by ID from the database
func GetThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) {
	if tprepo := getThirdPartyRepo(w, r); tprepo != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithJSONBody(w, ctxServices.Log, tprepo)
	}
}

// UpdateThirdPartyRepo updates the existing third party repository
func UpdateThirdPartyRepo(w http.ResponseWriter, r *http.Request) {
	oldtprepo := getThirdPartyRepo(w, r)
	if oldtprepo == nil {
		// error is handled by getThirdPartyRepo
		return
	}
	ctxServices := dependencies.ServicesFromContext(r.Context())
	tprepo, err := createRequest(w, r)
	if err != nil {
		// error handled by createRequest already
		return
	}
	err = ctxServices.ThirdPartyRepoService.UpdateThirdPartyRepo(tprepo, oldtprepo.OrgID, fmt.Sprint(oldtprepo.ID))
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryAlreadyExists, *services.ThirdPartyRepositoryImagesExists:
			apiError = errors.NewBadRequest(err.Error())
		case *services.ThirdPartyRepositoryNotFound:
			apiError = errors.NewNotFound(err.Error())
		default:
			apiError = errors.NewInternalServerError()
			apiError.SetTitle("failed to update custom repository")
		}
		respondWithAPIError(w, ctxServices.Log, apiError)
		return
	}

	repoDetails, err := ctxServices.ThirdPartyRepoService.GetThirdPartyRepoByID(fmt.Sprint(oldtprepo.ID))
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error getting custom repository")
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, repoDetails)
}

// DeleteThirdPartyRepoByID deletes the third party repository using ID
func DeleteThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) {
	tprepo := getThirdPartyRepo(w, r)
	if tprepo == nil {
		// error response handled by getThirdPartyRepo
		return
	}
	ctxServices := dependencies.ServicesFromContext(r.Context())
	tprepo, err := ctxServices.ThirdPartyRepoService.DeleteThirdPartyRepoByID(fmt.Sprint(tprepo.ID))
	if err != nil {
		var responseErr errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryNotFound:
			responseErr = errors.NewNotFound(err.Error())
		case *services.ThirdPartyRepositoryImagesExists:
			responseErr = errors.NewBadRequest(err.Error())
		default:
			responseErr = errors.NewInternalServerError()
			responseErr.SetTitle("failed to delete custom repository")
		}
		ctxServices.Log.WithField("error", err.Error()).Error("Error when deleting custom repository")
		respondWithAPIError(w, ctxServices.Log, responseErr)
		return
	}
	respondWithJSONBody(w, ctxServices.Log, tprepo)
}

func validateGetAllThirdPartyRepoFilterParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		var errs []validationError
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
		respondWithJSONBody(w, ctxServices.Log, &errs)
	})
}
