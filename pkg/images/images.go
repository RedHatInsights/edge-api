package images

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// MakeRouter adds support for operations on images
func MakeRouter(sub chi.Router) {
	sub.With(validateGetAllSearchParams).With(common.Paginate).Get("/", GetAll)
	sub.Post("/", Create)
	sub.Route("/{imageId}", func(r chi.Router) {
		r.Use(ImageCtx)
		r.Get("/", GetByID)
		r.Get("/status", GetStatusByID)
		r.Post("/installer", CreateInstallerForImage)
	})
}

// This provides type safety in the context object for our "image" key.  We
// _could_ use a string but we shouldn't just in case someone else decides that
// "image" would make the perfect key in the context object.  See the
// documentation: https://golang.org/pkg/context/#WithValue for further
// rationale.
type key int

const imageKey key = 1

var validStatuses = []string{models.ImageStatusCreated, models.ImageStatusBuilding, models.ImageStatusError, models.ImageStatusSuccess}

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
			result := db.DB.Where("images.account = ?", account).Joins("Commit").First(&image, id)
			if image.InstallerID != nil {
				result := db.DB.First(&image.Installer, image.InstallerID)
				if result.Error != nil {
					err := errors.NewInternalServerError()
					w.WriteHeader(err.Status)
					json.NewEncoder(w).Encode(&err)
					return
				}
			}
			err = db.DB.Model(&image.Commit).Association("Packages").Find(&image.Commit.Packages)
			if err != nil {
				err := errors.NewInternalServerError()
				w.WriteHeader(err.Status)
				json.NewEncoder(w).Encode(&err)
				return
			}
			if result.Error != nil {
				err := errors.NewNotFound(result.Error.Error())
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
type CreateImageRequest struct {
	// The image to create.
	//
	// in: body
	// required: true
	Image *models.Image
}

// Create creates an image on hosted image builder.
// It always creates a commit on Image Builder.
// We're creating a update on the background to transfer the commit to our repo.
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
	image, err = imagebuilder.Client.ComposeCommit(image, headers)
	if err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	tx = db.DB.Save(&image)
	if tx.Error != nil {
		log.Error(tx.Error)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	tx = db.DB.Save(&image.Commit)
	if tx.Error != nil {
		log.Error(tx.Error)
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
				log.Error(err)
				panic(err)
			}
			if i.Commit.Status != models.ImageStatusBuilding {
				break
			}
			time.Sleep(1 * time.Minute)
		}
		log.Infof("Commit %#v for Image %#v is ready. Creating OSTree repo.", i.Commit, image)
		err := commits.RepoBuilderInstance.ImportRepo(i.Commit)
		if err != nil {
			log.Error(err)
			return
		}
		log.Infof("OSTree repo %d for commit %d and Image %d is ready. ", i.Commit.ID, i.Commit.ID, i.ID)

		// TODO: This is also where we need to get the metadata from image builder
		// in a separate goroutine
		i.Status = models.ImageStatusSuccess
		db.DB.Save(&i)

		// TODO: We need to discuss this whole thist post-July deliverable
		if i.ImageType == models.ImageTypeInstaller {
			i.Installer = &models.Installer{
				Status:  models.ImageStatusBuilding,
				Account: i.Account,
			}
			i.Status = models.ImageStatusBuilding
			tx = db.DB.Save(&i)
			if tx.Error != nil {
				log.Error(err)
				return
			}
			tx = db.DB.Save(&i.Installer)
			i, err := imagebuilder.Client.ComposeInstaller(i.Commit, i, headers)
			if err != nil {
				log.Error(err)
				return
			}
			if tx.Error != nil {
				log.Error(err)
				json.NewEncoder(w).Encode(&err)
				return
			}

			for {
				i, err := updateImageStatus(i, r)
				if err != nil {
					panic(err)
				}
				if i.Installer.Status != models.ImageStatusBuilding {
					break
				}
				time.Sleep(1 * time.Minute)
			}

		}
	}(image.ID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)
}

var imageFilters = common.ComposeFilters(
	common.OneOfFilterHandler(&common.Filter{
		QueryParam: "status",
		DBField:    "images.status",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "name",
		DBField:    "images.name",
	}),
	common.ContainFilterHandler(&common.Filter{
		QueryParam: "distribution",
		DBField:    "images.distribution",
	}),
	common.CreatedAtFilterHandler(&common.Filter{
		QueryParam: "created_at",
		DBField:    "images.created_at",
	}),
	common.SortFilterHandler("images", "created_at", "DESC"),
)

type validationError struct {
	Key    string
	Reason string
}

func validateGetAllSearchParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs := []validationError{}
		if statuses, ok := r.URL.Query()["status"]; ok {
			for _, status := range statuses {
				if status != models.ImageStatusCreated && status != models.ImageStatusBuilding && status != models.ImageStatusError && status != models.ImageStatusSuccess {
					errs = append(errs, validationError{Key: "status", Reason: fmt.Sprintf("%s is not a valid status. Status must be %s", status, strings.Join(validStatuses, " or "))})
				}
			}
		}
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
			if name != "status" && name != "name" && name != "distribution" && name != "created_at" {
				errs = append(errs, validationError{Key: "sort_by", Reason: fmt.Sprintf("%s is not a valid sort_by. Sort-by must be status or name or distribution or created_at", name)})
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
	result = result.Limit(pagination.Limit).Offset(pagination.Offset).Where("images.account = ?", account).Joins("Commit").Joins("Installer").Find(&images)
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
	log.Info("Requesting image status on image builder aa")
	headers := common.GetOutgoingHeaders(r)
	if image.Commit.Status == models.ImageStatusBuilding {
		image, err := imagebuilder.Client.GetCommitStatus(image, headers)
		if err != nil {
			return image, err
		}
		if image.Commit.Status != models.ImageStatusBuilding && image.Installer == nil {
			tx := db.DB.Save(&image.Commit)
			if tx.Error != nil {
				return image, tx.Error
			}
		}
	}
	if image.Installer != nil && image.Installer.Status == models.ImageStatusBuilding {
		image, err := imagebuilder.Client.GetInstallerStatus(image, headers)
		if err != nil {
			return image, err
		}
		if image.Installer.Status != models.ImageStatusBuilding {
			tx := db.DB.Save(&image.Installer)
			if tx.Error != nil {
				return image, tx.Error
			}
		}
	}
	if image.Status != models.ImageStatusBuilding {
		tx := db.DB.Save(&image)
		if tx.Error != nil {
			return image, tx.Error
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
			Name   string
			ID     uint
		}{
			image.Status,
			image.Name,
			image.ID,
		})
	}
}

// GetByID obtains a image from the database for an account
func GetByID(w http.ResponseWriter, r *http.Request) {
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
		json.NewEncoder(w).Encode(image)
	}
}

// CreateInstallerForImage creates a installer for a Image
// It requires a created image and an update for the commit
func CreateInstallerForImage(w http.ResponseWriter, r *http.Request) {
	image := getImage(w, r)
	image.Installer = &models.Installer{
		Status:  models.ImageStatusCreated,
		Account: image.Account,
	}
	image.ImageType = models.ImageTypeInstaller
	tx := db.DB.Save(&image)
	if tx.Error != nil {
		log.Error(tx.Error)
		err := errors.NewInternalServerError()
		err.Title = "Failed saving image status"
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	headers := common.GetOutgoingHeaders(r)
	var repo *models.Repo
	result := db.DB.Where("ID = ?", image.Commit.ID).Take(&repo)
	if result.Error != nil {
		err := errors.NewBadRequest(fmt.Sprintf("Commit Repo wasn't found in the database: #%v", image.Commit))
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image, err := imagebuilder.Client.ComposeInstaller(image.Commit, image, headers)
	if err != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	image.Installer.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	tx = db.DB.Save(&image)
	if tx.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}
	tx = db.DB.Save(&image.Installer)
	if tx.Error != nil {
		log.Error(err)
		err := errors.NewInternalServerError()
		w.WriteHeader(err.Status)
		json.NewEncoder(w).Encode(&err)
		return
	}

	go func(id uint) {
		var i *models.Image
		db.DB.Joins("Commit").Joins("Installer").First(&i, id)
		for {
			i, err := updateImageStatus(i, r)
			if err != nil {
				panic(err)
			}
			if i.Installer.Status != models.ImageStatusBuilding {
				break
			}
			time.Sleep(1 * time.Minute)
		}
	}(image.ID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&image)
}
