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

type OwnershipVoucherService struct {
	ctx context.Context
	log *log.Entry
}

type OwnershipVoucherServiceInterface interface {
	ParseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error)
	CreateFDOClient() *fdo.Client
}

func NewOwnershipVoucherService(ctx context.Context, log *log.Entry) OwnershipVoucherServiceInterface {
	return &OwnershipVoucherService{
		ctx: ctx,
		log: log.WithField("service", "ownershipvoucher"),
	}
}

// ParseVouchers parses vouchers from a byte array, returning the data and error if any
func (ovs *OwnershipVoucherService) ParseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error) {
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

func (ovs *OwnershipVoucherService) CreateFDOClient() *fdo.Client {
	return fdo.InitClient(ovs.ctx, ovs.log)
}
