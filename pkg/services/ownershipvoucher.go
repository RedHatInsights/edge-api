//go:build !fdo
// +build !fdo

package services

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// OwnershipVoucherServiceInterface is empty for non-fdo builds
type OwnershipVoucherServiceInterface interface{}

// NewOwnershipVoucherService returns nil for non-fdo builds
func NewOwnershipVoucherService(ctx context.Context, log *log.Entry) OwnershipVoucherServiceInterface {
	return nil
}
