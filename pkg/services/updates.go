package services

// UpdateServiceInterface defines the interface that helps
// handle the business logic of sending updates to a edge device
type UpdateServiceInterface interface {
}

// NewUpdateService gives a instance of the main implementation of a UpdateServiceInterface
func NewUpdateService() UpdateServiceInterface {
	return &UpdateService{}
}

// UpdateService is the main implementation of a UpdateServiceInterface
type UpdateService struct{}
