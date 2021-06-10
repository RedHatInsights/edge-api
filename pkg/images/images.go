package images

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
)

// MakeRouter adds support for operations on images
func MakeRouter(sub chi.Router) {
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
	var image models.Image
	if err := json.NewDecoder(r.Body).Decode(&image); err != nil {
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	if err := image.ValidateRequest(); err != nil {
		err := errors.NewBadRequest(err.Error())
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}

}
