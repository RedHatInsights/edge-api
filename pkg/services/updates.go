package services

type UpdatesServiceInterface interface {
}

type UpdatesService struct {
	commitService CommitServiceInterface
}

func NewUpdatesService() UpdatesServiceInterface {
	return &UpdatesService{
		commitService: NewCommitService(),
	}
}
