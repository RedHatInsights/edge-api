// FIXME: golangci-lint
// nolint:revive
package services

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// Service is a blueprint for a service
type Service struct {
	ctx context.Context
	log log.FieldLogger
}

// ServiceInterface defines the interface for a service
type ServiceInterface interface{}

// NewService creates a new service
func NewService(ctx context.Context, log log.FieldLogger) Service {
	return Service{ctx: ctx, log: log}
}

// SetLog sets a new log on the struct
func (s *Service) SetLog(log log.FieldLogger) {
	s.log = log
}
