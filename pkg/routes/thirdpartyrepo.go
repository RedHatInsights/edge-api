// FIXME: golangci-lint
package routes // nolint:revive

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"gorm.io/gorm"
)

type tprepoTypeKey int

const tprepoKey tprepoTypeKey = iota

// MakeThirdPartyRepoRouter adds support for operation on ThirdPartyRepo
func MakeThirdPartyRepoRouter(sub chi.Router) {
	sub.With(ValidateQueryParams("thirdpartyrepo")).With(validateGetAllThirdPartyRepoFilterParams).With(common.Paginate).Get("/", GetAllThirdPartyRepo) // nolint:revive
	sub.Post("/", CreateThirdPartyRepo)
	sub.Route("/{ID}", func(r chi.Router) {
		r.Use(ThirdPartyRepoCtx)
		r.Get("/", GetThirdPartyRepoByID) // nolint:revive
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

func getThirdPartyRepo(w http.ResponseWriter, r *http.Request) *models.ThirdPartyRepo { // nolint:revive
	ctx := r.Context()
	ctxServices := dependencies.ServicesFromContext(ctx)
	tprepo, ok := ctx.Value(tprepoKey).(*models.ThirdPartyRepo)
	if !ok {
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Failed getting custom repository from context")) // nolint:revive
		return nil
	}
	return tprepo
}

// CreateThirdPartyRepo creates Third Party Repository
func CreateThirdPartyRepo(w http.ResponseWriter, r *http.Request) { // nolint:revive
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

	thirdPartyRepo, err = ctxServices.ThirdPartyRepoService.CreateThirdPartyRepo(thirdPartyRepo, orgID) // nolint:revive
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryNameIsEmpty, *services.ThirdPartyRepositoryURLIsEmpty, *services.ThirdPartyRepositoryAlreadyExists: // nolint:revive
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
func createRequest(w http.ResponseWriter, r *http.Request) (*models.ThirdPartyRepo, error) { // nolint:revive
	ctxServices := dependencies.ServicesFromContext(r.Context())

	var tprepo *models.ThirdPartyRepo
	if err := readRequestJSONBody(w, r, ctxServices.Log, &tprepo); err != nil {
		return nil, err
	}

	if err := tprepo.ValidateRequest(); err != nil {
		ctxServices.Log.WithField("error", err.Error()).Info("custom repository validation error") // nolint:revive
		respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error()))                 // nolint:revive
		return nil, err
	}
	return tprepo, nil
}

// GetAllThirdPartyRepo return all the ThirdPartyRepo
func GetAllThirdPartyRepo(w http.ResponseWriter, r *http.Request) { // nolint:revive
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
	if imageID != "" { // nolint:revive
		ctx = db.Org(orgID, "").Debug().
			Joins("left join images_repos on third_party_repo_id = id and image_id = ?", imageID). // nolint:revive
			Order("images_repos.third_party_repo_id DESC NULLS LAST").
			Model(&models.ThirdPartyRepo{})
		ctx = thirdPartyRepoFilters(r, ctx)
	} else {
		ctx = db.OrgDB(orgID, thirdPartyRepoFilters(r, db.DB), "").Debug().Model(&models.ThirdPartyRepo{}) // nolint:revive
	}

	// Check to see if feature is enabled and not in ephemeral
	cfg := config.Get()
	if cfg.FeatureFlagsEnvironment != "ephemeral" && cfg.FeatureFlagsURL != "" {
		enabled := feature.CheckFeature(feature.FeatureCustomRepos)
		if !enabled {
			respondWithAPIError(w, ctxServices.Log, errors.NewFeatureNotAvailable("Feature not available")) // nolint:revive
			return
		}
	}

	pagination := common.GetPagination(r)

	if result := ctx.Count(&count); result.Error != nil {
		ctxServices.Log.WithField("error", result.Error).Error("Error counting results") // nolint:revive
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}

	if imageID != "" {
		if result := ctx.Preload("Images", "id = ?", imageID).Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo); result.Error != nil { // nolint:revive
			ctxServices.Log.WithField("error", result.Error).Error("Error returning results") // nolint:revive
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())          // nolint:revive
			return
		}
	} else {
		if result := ctx.Limit(pagination.Limit).Offset(pagination.Offset).Find(&tprepo); result.Error != nil { // nolint:revive
			ctxServices.Log.WithField("error", result.Error).Error("Error returning results") // nolint:revive
			respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())          // nolint:revive
			return
		}
	}

	respondWithJSONBody(w, ctxServices.Log, map[string]interface{}{"data": &tprepo, "count": count}) // nolint:revive
}

// ThirdPartyRepoCtx is a handler to Third Party Repository requests
func ThirdPartyRepoCtx(next http.Handler) http.Handler { // nolint:revive
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // nolint:revive
		ctxServices := dependencies.ServicesFromContext(r.Context())
		if ID := chi.URLParam(r, "ID"); ID != "" { // nolint:gocritic,revive
			_, err := strconv.Atoi(ID)
			ctxServices.Log = ctxServices.Log.WithField("thirdPartyRepoID", ID)
			ctxServices.Log.Debug("Retrieving custom repository")
			if err != nil {
				ctxServices.Log.Debug("ID is not an integer")
				respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest(err.Error())) // nolint:revive
				return
			}

			tprepo, err := ctxServices.ThirdPartyRepoService.GetThirdPartyRepoByID(ID) // nolint:revive
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
			ctxServices.Log.Debug("custom repository ID was not passed to the request or it was empty")       // nolint:revive
			respondWithAPIError(w, ctxServices.Log, errors.NewBadRequest("Custom repository ID is required")) // nolint:revive
			return
		}
	})
}

// GetThirdPartyRepoByID gets the Third Party repository by ID from the database
func GetThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) { // nolint:revive
	if tprepo := getThirdPartyRepo(w, r); tprepo != nil {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		respondWithJSONBody(w, ctxServices.Log, tprepo)
	}
}

// UpdateThirdPartyRepo updates the existing third party repository
func UpdateThirdPartyRepo(w http.ResponseWriter, r *http.Request) { // nolint:revive
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
	err = ctxServices.ThirdPartyRepoService.UpdateThirdPartyRepo(tprepo, oldtprepo.OrgID, fmt.Sprint(oldtprepo.ID)) // nolint:revive
	if err != nil {
		var apiError errors.APIError
		switch err.(type) {
		case *services.ThirdPartyRepositoryAlreadyExists, *services.ThirdPartyRepositoryImagesExists: // nolint:revive
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

	repoDetails, err := ctxServices.ThirdPartyRepoService.GetThirdPartyRepoByID(fmt.Sprint(oldtprepo.ID)) // nolint:revive
	if err != nil {
		ctxServices.Log.WithField("error", err.Error()).Error("Error getting custom repository") // nolint:revive
		respondWithAPIError(w, ctxServices.Log, errors.NewInternalServerError())
		return
	}
	w.WriteHeader(http.StatusOK)
	respondWithJSONBody(w, ctxServices.Log, repoDetails)
}

// DeleteThirdPartyRepoByID deletes the third party repository using ID
func DeleteThirdPartyRepoByID(w http.ResponseWriter, r *http.Request) { // nolint:revive
	tprepo := getThirdPartyRepo(w, r)
	if tprepo == nil {
		// error response handled by getThirdPartyRepo
		return
	}
	ctxServices := dependencies.ServicesFromContext(r.Context())
	tprepo, err := ctxServices.ThirdPartyRepoService.DeleteThirdPartyRepoByID(fmt.Sprint(tprepo.ID)) // nolint:revive
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
		ctxServices.Log.WithField("error", err.Error()).Error("Error when deleting custom repository") // nolint:revive
		respondWithAPIError(w, ctxServices.Log, responseErr)
		return
	}
	respondWithJSONBody(w, ctxServices.Log, tprepo)
}

func validateGetAllThirdPartyRepoFilterParams(next http.Handler) http.Handler { // nolint:revive
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxServices := dependencies.ServicesFromContext(r.Context())
		var errs []validationError
		if val := r.URL.Query().Get("created_at"); val != "" {
			if _, err := time.Parse(common.LayoutISO, val); err != nil {
				errs = append(errs, validationError{Key: "created_at", Reason: err.Error()}) // nolint:revive
			}
		}
		if val := r.URL.Query().Get("sort_by"); val != "" {
			name := val
			if string(val[0]) == "-" { // nolint:revive
				name = val[1:] // nolint:revive
			}
			if name != "name" && name != "created_at" && name != "updated_at" { // nolint:revive
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be name or created_at or updated_at", name)}) // nolint:revive
			}
		}

		if len(errs) == 0 { // nolint:revive
			next.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		respondWithJSONBody(w, ctxServices.Log, &errs)
	})
}
