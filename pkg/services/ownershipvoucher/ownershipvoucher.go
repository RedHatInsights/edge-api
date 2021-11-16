package ownershipvoucher

// #cgo LDFLAGS: -l:libfdo_data.so.0
// #include <stdlib.h>
// #include <fdo_data.h>
import "C"
import (
	"context"
	"errors"
	"github.com/redhatinsights/edge-api/pkg/clients/fdo"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// Service for ownership voucher management
type Service struct {
	ctx context.Context
	log *log.Entry
}

// ServiceInterface is the interface for the ownership voucher service
type ServiceInterface interface {
	ParseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error)
	CreateFDOClient() *fdo.Client
}

// NewService creates a new ownership voucher service
func NewService(ctx context.Context, log *log.Entry) ServiceInterface {
	return &Service{
		ctx: ctx,
		log: log.WithField("service", "ownershipvoucher"),
	}
}

// ParseVouchers parses vouchers from a byte array, returning the data and error if any
func (ovs *Service) ParseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error) {
	voucherBytesLen := C.size_t(len(voucherBytes))
	voucherCBytes := C.CBytes(voucherBytes)
	defer C.free(voucherCBytes)

	voucher := C.fdo_ownershipvoucher_from_data(voucherCBytes, voucherBytesLen)
	defer C.fdo_ownershipvoucher_free(voucher)
	if voucher == nil {
		ovs.log.Error("Failed to parse ownership voucher")
		return []models.OwnershipVoucherData{}, errors.New("failed to parse ownership voucher")
	}

	guidC := C.fdo_ownershipvoucher_header_get_guid(voucher)
	defer C.fdo_free_string(guidC)
	guid := C.GoString(guidC)

	devinfoC := C.fdo_ownershipvoucher_header_get_device_info_string(voucher)
	defer C.fdo_free_string(devinfoC)
	devinfo := C.GoString(devinfoC)

	return []models.OwnershipVoucherData{
		models.OwnershipVoucherData{
			ProtocolVersion: uint(C.fdo_ownershipvoucher_header_get_protocol_version(voucher)),
			GUID:            guid,
			DeviceName:      devinfo,
		},
	}, nil
}

// CreateFDOClient creates a new FDO client
func (ovs *Service) CreateFDOClient() *fdo.Client {
	return fdo.InitClient(ovs.ctx, ovs.log)
}
