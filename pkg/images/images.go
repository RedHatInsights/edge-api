package images

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// MakeRouter adds support for operations on images
func MakeRouter(sub chi.Router) {
	sub.With(common.Paginate).Get("/", GetAll)
	sub.Post("/", Create)
	sub.Route("/{imageId}", func(r chi.Router) {
		r.Use(ImageCtx)
		r.Get("/status", GetStatusByID)
	})
}

// This provides type safety in the context object for our "image" key.  We
// _could_ use a string but we shouldn't just in case someone else decides that
// "image" would make the perfect key in the context object.  See the
// documentation: https://golang.org/pkg/context/#WithValue for further
// rationale.
type key int

const imageKey key = 1

// ImageCtx is a handler for Image requests
func ImageCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var image models.Image
		account, err := common.GetAccount(r)
		if err != nil {
			err := errors.NewBadRequest(err.Error())
			w.WriteHeader(err.Status)
			json.NewEncoder(w).Encode(&err)
			return
		}
		if imageID := chi.URLParam(r, "imageId"); imageID != "" {
			id, err := strconv.Atoi(imageID)
			if err != nil {
				err := errors.NewBadRequest(err.Error())
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			result := db.DB.Where("account = ?", account).First(&image, id)
			if result.Error != nil {
				err := errors.NewNotFound(err.Error())
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			result = db.DB.Where("account = ?", account).First(&image.Commit, image.CommitID)
			if result.Error != nil {
				err := errors.NewNotFound(err.Error())
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			ctx := context.WithValue(r.Context(), imageKey, &image)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// A CreateImageRequest model.
//
// This is used as the body for the Create image request.
// swagger:parameters createImage
type CreateImageRequest struct {
	// The image to create.
	//
	// in: body
	// required: true
	Image *models.Image
}

// Create swagger:route POST /images image createImage
//
// Creates an image on hosted image builder.
//
// It is used to create a new image on the hosted image builder.
// Responses:
//   200: image
//   500: genericError
//   400: badRequest
func Create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var image *models.Image
	if err := json.NewDecoder(r.Body).Decode(&image); err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	if err := image.ValidateRequest(); err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image.Account = account
	image.Commit.Account = account
	image.Status = models.ImageStatusCreated
	tx := db.DB.Create(&image)
	if tx.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		err.Title = "Failed creating image compose"
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	headers := common.GetOutgoingHeaders(r)
	image, err = imagebuilder.Client.Compose(image, headers)
	if err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image.Status = models.ImageStatusBuilding
	tx = db.DB.Save(&image)
	if tx.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}

	go func(id uint) {
		var i *models.Image
		db.DB.Joins("Commit").First(&i, id)
		for {
			i, err := updateImageStatus(i, r)
			if err != nil {
				panic(err)
			}
			if i.Status != models.ImageStatusBuilding {
				break
			}
		}
	}(image.ID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)
}

var imageFilters = common.ComposeFilters(
	common.OneOfFilterHandler("status"),
	common.OneOfFilterHandler("image_type"),
	common.ContainFilterHandler("name"),
	common.ContainFilterHandler("distribution"),
	common.CreatedAtFilterHandler(),
	common.SortFilterHandler("id", "ASC"),
)

// GetAll image objects from the database for an account
func GetAll(w http.ResponseWriter, r *http.Request) {
	var images []models.Image
	result := imageFilters(r, db.DB)
	pagination := common.GetPagination(r)
	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("account = ?", account).Find(&images)
	if result.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	json.NewEncoder(w).Encode(&images)
}

func getImage(w http.ResponseWriter, r *http.Request) *models.Image {
	ctx := r.Context()
	image, ok := ctx.Value(imageKey).(*models.Image)
	if !ok {
		err := errors.NewBadRequest("Must pass image id")
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return nil
	}
	return image
}

func updateImageStatus(image *models.Image, r *http.Request) (*models.Image, error) {
	log.Info("Requesting image status on image builder")
	headers := common.GetOutgoingHeaders(r)
	image, err := imagebuilder.Client.GetStatus(image, headers)
	if err != nil {
		return image, err
	}
	if image.Status != models.ImageStatusBuilding {
		tx := db.DB.Save(&image.Commit)
		if tx.Error != nil {
			return image, err
		}
		tx = db.DB.Save(&image)
		if tx.Error != nil {
			return image, err
		}
	}
	return image, nil
}

// GetStatusByID returns the image status. If still building, goes to image builder API.
func GetStatusByID(w http.ResponseWriter, r *http.Request) {
	if image := getImage(w, r); image != nil {
		if image.Status == models.ImageStatusBuilding {
			var err error
			image, err = updateImageStatus(image, r)
			if err != nil {
				log.Error(err)
				err := errors.NewInternalServerError()
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
		}
		json.NewEncoder(w).Encode(struct {
			Status string
		}{
			image.Status,
		})
	}
}
