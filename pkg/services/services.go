package services

import (
	"context"
	log "github.com/sirupsen/logrus"
)

// Blueprint for a service
type Service struct {
	ctx context.Context
	log *log.Entry
}

// Blueprint for a service interface
type ServiceInterface interface{}

// NewService creates a new service pointer
func NewService(ctx context.Context, log *log.Entry) ServiceInterface {
	return &Service{ctx: ctx, log: log}
}
