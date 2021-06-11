package images

import (
	"encoding/json"
	"net/http"

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
	sub.Get("/", GetAll)
	sub.Post("/", Create)
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

	image, err := imagebuilder.Client.Compose(image)
	if err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image.Account, err = common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image.Commit.Account = image.Account
	tx := db.DB.Create(&image)
	if tx.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)
}

// GetAll image objects from the database for an account
func GetAll(w http.ResponseWriter, r *http.Request) {
	var images []models.Image
	account, err := common.GetAccount(r)
	if err != nil {
		log.Info(err)
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	result := db.DB.Where("account = ?", account).Find(&images)
	if result.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}

	json.NewEncoder(w).Encode(&images)
}
