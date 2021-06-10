package images

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/errors"
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
	Distribution string // rhel-8
	Architecture string // x86_64
	OSTreeRef    string // "rhel/8/x86_64/edge"
	OSTreeURL    string
	Description  string
	OutputType   string // ISO/TAR
	Packages     []string
	Status       string
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
	var image Image
	err := json.NewDecoder(r.Body).Decode(&image)
	if err != nil {
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	fmt.Println(image)
}
