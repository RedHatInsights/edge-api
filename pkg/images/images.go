package images

import (
	"net/http"

	"github.com/go-chi/chi"
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
	Image *Image
}

// An Image is what generates a OSTree Commit.
//
// swagger:model image
type Image struct {
	ReleaseName string // AKA: OSTree ref
	Description string
	OutputType  string // ISO/TAR
	Packages    []string
	Status      string
}

// Create swagger:route POST /images image createImage
//
// Creates an image on hosted image builder.
//
// It is used to create a new image on the hosted image builder.
// Responses:
//   200: image
func Create(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
