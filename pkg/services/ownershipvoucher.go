package services

// #cgo LDFLAGS: -l:libfdo_data.so.0
// #include <stdlib.h>
// #include <fdo_data.h>
import "C"
import (
	"context"
	"errors"

	"github.com/redhatinsights/edge-api/pkg/clients/fdo"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	log "github.com/sirupsen/logrus"
)

// OwnershipVoucherService for ownership voucher management
type OwnershipVoucherService struct {
	ctx context.Context
	log *log.Entry
}

// OwnershipVoucherServiceInterface is the interface for the ownership voucher service
type OwnershipVoucherServiceInterface interface {
	BatchUploadOwnershipVouchers(voucherBytes []byte, numOfOVs uint) (interface{}, error)
	BatchDeleteOwnershipVouchers(fdoUUIDList []string) (interface{}, error)
	ConnectDevices(fdoUUIDList []string) ([]interface{}, []error)
	ParseOwnershipVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error)
	GetOwnershipVoucherByGUID(OwnershipVoucherGUID string) (*models.OwnershipVoucherData, error)
	storeOwnershipVouchers(data []models.OwnershipVoucherData)
	removeOwnershipVouchers(fdoUUIDList []string)
	parseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error)
	createFDOClient() *fdo.Client
}

// NewOwnershipVoucherService creates a new ownership voucher service
func NewOwnershipVoucherService(ctx context.Context, log *log.Entry) OwnershipVoucherServiceInterface {
	return &OwnershipVoucherService{
		ctx: ctx,
		log: log.WithField("service", "ownershipvoucher"),
	}
}

// BatchUploadOwnershipVouchers creates empty devices with ownership vouchers data
func (ovs *OwnershipVoucherService) BatchUploadOwnershipVouchers(voucherBytes []byte, numOfOVs uint) (interface{}, error) {
	logFields := log.Fields{"method": "services.BatchUploadOwnershipVouchers"}
	ovs.log.WithFields(logFields).Debug("Creating ownership vouchers")
	data, err := ovs.ParseOwnershipVouchers(voucherBytes)
	if err != nil {
		ovs.log.WithFields(logFields).Error("Failed to parse ownership vouchers")
		return nil, err
	}
	ovs.log.WithFields(logFields).Debug("Creating FDO client")
	fdoClient := ovs.createFDOClient()
	resp, err := fdoClient.BatchUpload(voucherBytes, numOfOVs)
	if err != nil {
		ovs.log.WithFields(logFields).Error("Failed to upload ownership vouchers to the FDO server")
		return resp, err
	}
	ovs.storeOwnershipVouchers(data)
	return resp, nil
}

// BatchDeleteOwnershipVouchers deletes ownership vouchers from the FDO server
func (ovs *OwnershipVoucherService) BatchDeleteOwnershipVouchers(fdoUUIDList []string) (interface{}, error) {
	logFields := log.Fields{"method": "services.BatchDeleteOwnershipVouchers"}
	ovs.log.WithFields(logFields).Debug("Deleting ownership vouchers")
	fdoClient := ovs.createFDOClient()
	resp, err := fdoClient.BatchDelete(fdoUUIDList)
	ovs.removeOwnershipVouchers(fdoUUIDList)
	return resp, err
}

// ConnectDevices API point for the FDO server to connect devices
func (ovs *OwnershipVoucherService) ConnectDevices(fdoUUIDList []string) (resp []interface{}, errList []error) {
	logFields := log.Fields{"method": "services.ConnectDevices"}
	ovs.log.WithFields(logFields).Debug("Connecting devices")
	for _, guid := range fdoUUIDList {
		ov, err := ovs.GetOwnershipVoucherByGUID(guid)
		if err != nil {
			ovs.log.WithFields(logFields).Warn("Couldn't find OwnershipVoucher ", guid, err)
			errList = append(errList, errors.New(guid))
		} else {
			ov.DeviceConnected = true
			resp = append(resp, map[string]string{"guid": guid})
			db.DB.Save(&ov)
		}
	}
	return
}

// ParseOwnershipVouchers reads ownership vouchers from bytes
func (ovs *OwnershipVoucherService) ParseOwnershipVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error) {
	logFields := log.Fields{"method": "services.ParseOwnershipVouchers"}
	ovs.log.WithFields(logFields).Debug("Parsing ownership vouchers")
	data, err := ovs.parseVouchers(voucherBytes)
	if err != nil {
		ovs.log.WithFields(logFields).Error("Failed to parse ownership vouchers")
		return nil, err
	}
	return data, nil
}

// GetOwnershipVoucherByGUID receives GUID string and get a *models.OwnershipVoucherData back
func (ovs *OwnershipVoucherService) GetOwnershipVoucherByGUID(OwnershipVoucherGUID string) (*models.OwnershipVoucherData, error) {
	logFields := log.Fields{"method": "services.GetOwnershipVoucherByGUID"}
	ovs.log.WithFields(logFields).Debug("Getting ownership voucher by GUID")
	var ov models.OwnershipVoucherData
	result := db.DB.Where("guid = ?", OwnershipVoucherGUID).First(&ov)
	if result.Error != nil {
		return nil, result.Error
	}
	return &ov, nil
}

// storeOwnershipVouchers stores ownership vouchers to the database
func (ovs *OwnershipVoucherService) storeOwnershipVouchers(data []models.OwnershipVoucherData) {
	logFields := log.Fields{"method": "services.storeOwnershipVouchers"}
	ovs.log.WithFields(logFields).Debug("Store empty devices, with FDO info")
	for _, voucherData := range data {
		// create if not exist
		db.DB.Where("guid = ?", voucherData.GUID).FirstOrCreate(&voucherData)
	}
}

// removeOwnershipVouchers removes ownership vouchers from the database
func (ovs *OwnershipVoucherService) removeOwnershipVouchers(fdoUUIDList []string) {
	logFields := log.Fields{"method": "services.removeOwnershipVouchers"}
	for _, guid := range fdoUUIDList {
		ov, err := ovs.GetOwnershipVoucherByGUID(guid)
		if err != nil {
			ovs.log.WithFields(logFields).Error("Failed to get ownership voucher by GUID ", guid)
			continue
		}
		// remove completely
		db.DB.Unscoped().Delete(&models.OwnershipVoucherData{}, "guid = ?", ov.GUID)
	}
}

// parseVouchers parses vouchers from a byte array, returning the data and error if any
func (ovs *OwnershipVoucherService) parseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error) {
	voucherBytesLen := C.size_t(len(voucherBytes))
	voucherCBytes := C.CBytes(voucherBytes)
	defer C.free(voucherCBytes)

	voucher := C.fdo_ownershipvoucher_from_data(voucherCBytes, voucherBytesLen)
	defer C.fdo_ownershipvoucher_free(voucher)
	if voucher == nil {
		ovs.log.WithField("method", "services.parseVouchers").Error("Failed to parse ownership voucher")
		return nil, errors.New("failed to parse ownership voucher")
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

// createFDOClient creates a new FDO client
func (ovs *OwnershipVoucherService) createFDOClient() *fdo.Client {
	return fdo.InitClient(ovs.ctx, ovs.log)
}
