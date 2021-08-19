package services

// ImageServiceInterface defines the interface that helps handle
// the business logic of creating RHEL For Edge Images
type ImageServiceInterface interface {
}

// NewImageService gives a instance of the main implementation of a ImageServiceInterface
func NewImageService() ImageServiceInterface {
	return &ImageService{}
}

// ImageService is the main implementation of a ImageServiceInterface
type ImageService struct {
}
