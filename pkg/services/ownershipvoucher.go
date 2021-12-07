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
	"gorm.io/gorm"

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
	GetFDODeviceByGUID(ownershipVoucherGUID string) (*models.FDODevice, error)
	GetOwnershipVouchersByGUID(ownershipVoucherGUID string) (*models.OwnershipVoucherData, error)
	GetFDOUserByGUID(ownershipVoucherGUID string) (*models.FDOUser, error)
	storeFDODevices(data []models.OwnershipVoucherData)
	removeFDODevices(fdoUUIDList []string)
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
	ovs.storeFDODevices(data)
	return resp, nil
}

// BatchDeleteOwnershipVouchers deletes ownership vouchers from the FDO server
func (ovs *OwnershipVoucherService) BatchDeleteOwnershipVouchers(fdoUUIDList []string) (interface{}, error) {
	logFields := log.Fields{"method": "services.BatchDeleteOwnershipVouchers"}
	ovs.log.WithFields(logFields).Debug("Deleting ownership vouchers")
	fdoClient := ovs.createFDOClient()
	resp, err := fdoClient.BatchDelete(fdoUUIDList)
	ovs.removeFDODevices(fdoUUIDList)
	return resp, err
}

// ConnectDevices API point for the FDO server to connect devices
func (ovs *OwnershipVoucherService) ConnectDevices(fdoUUIDList []string) (resp []interface{}, errList []error) {
	logFields := log.Fields{"method": "services.ConnectDevices"}
	ovs.log.WithFields(logFields).Debug("Connecting devices")
	for _, guid := range fdoUUIDList {
		fdoDevice, err := ovs.GetFDODeviceByGUID(guid)
		if err != nil {
			ovs.log.WithFields(logFields).Warn("Couldn't find OwnershipVoucher ", guid, err)
			errList = append(errList, errors.New(guid))
		} else {
			fdoDevice.Connected = true
			resp = append(resp, map[string]string{"guid": guid})
			db.DB.Save(&fdoDevice)
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

// GetFDODeviceByGUID receives GUID string and get a *models.FDODevice back
func (ovs *OwnershipVoucherService) GetFDODeviceByGUID(ownershipVoucherGUID string) (*models.FDODevice, error) {
	logFields := log.Fields{"method": "services.GetFDODeviceByGUID"}
	ovs.log.WithFields(logFields).Debug("Getting FDO device by GUID")
	// get the FDO device from the database
	var fdoDevice models.FDODevice
	result := joinWithFDODevices(ownershipVoucherGUID).First(&fdoDevice)
	if result.Error != nil {
		ovs.log.WithFields(logFields).Error("Failed to get FDO device by GUID ", result.Error)
		return nil, result.Error
	}
	// get the ownership voucher data related to the FDO device
	ov, err := ovs.GetOwnershipVouchersByGUID(ownershipVoucherGUID)
	if err != nil {
		ovs.log.WithFields(logFields).Error("Failed to get ownership voucher by GUID ", err)
		return nil, err
	}
	fdoDevice.OwnershipVoucherData = ov

	// get the FDO user related to the FDO device
	fdoUser, err := ovs.GetFDOUserByGUID(ownershipVoucherGUID)
	if err != nil {
		ovs.log.WithFields(logFields).Error("Failed to get FDO user by FDO device ", err)
		return nil, err
	}
	fdoDevice.InitialUser = fdoUser
	return &fdoDevice, nil
}

// GetOwnershipVouchersByGUID receives GUID string and get a *models.OwnershipVoucherData back
func (ovs *OwnershipVoucherService) GetOwnershipVouchersByGUID(ownershipVoucherGUID string) (*models.OwnershipVoucherData, error) {
	logFields := log.Fields{"method": "services.GetOwnershipVouchersByGUID"}
	ovs.log.WithFields(logFields).Debug("Getting ownership vouchers by GUID")
	var ov models.OwnershipVoucherData
	result := db.DB.Where("guid = ?", ownershipVoucherGUID).First(&ov)
	if result.Error != nil {
		ovs.log.WithFields(logFields).Error("Failed to get ownership vouchers by GUID ", result.Error)
		return nil, result.Error
	}
	return &ov, nil
}

// GetFDOUserByGUID receives an ownership voucher GUID and get a *models.FDOUser back
func (ovs *OwnershipVoucherService) GetFDOUserByGUID(ownershipVoucherGUID string) (*models.FDOUser, error) {
	logFields := log.Fields{"method": "services.GetFDOUserByFDODevice"}
	ovs.log.WithFields(logFields).Debug("Getting FDO user by FDO device")
	var fdoUser models.FDOUser
	result := db.DB.Joins("JOIN ownership_voucher_data ON ownership_voucher_data.fdo_device_id = fdo_users.fdo_device_id and ownership_voucher_data.guid = ?",
		ownershipVoucherGUID).First(&fdoUser)
	if result.Error != nil {
		ovs.log.WithFields(logFields).Error("Failed to get FDO user by FDO device ", result.Error)
		return nil, result.Error
	}
	return &fdoUser, nil
}

// storeFDODevices stores FDO devices to the database
func (ovs *OwnershipVoucherService) storeFDODevices(data []models.OwnershipVoucherData) {
	logFields := log.Fields{"method": "services.storeFDODevices"}
	ovs.log.WithFields(logFields).Debug("Store empty devices, with FDO info")
	for _, voucherData := range data {
		fdoDevice := models.FDODevice{
			OwnershipVoucherData: &voucherData,
			InitialUser:          &models.FDOUser{},
		}
		result := joinWithFDODevices(voucherData.GUID).FirstOrCreate(&fdoDevice)
		if result.Error != nil {
			ovs.log.WithFields(logFields).Error("Failed to store FDO device ", result.Error)
		}
	}
}

// removeFDODevices removes FDO devices from the database
func (ovs *OwnershipVoucherService) removeFDODevices(fdoUUIDList []string) {
	logFields := log.Fields{"method": "services.removeFDODevices"}
	for _, guid := range fdoUUIDList {
		// Delete the OwnershipVoucherData associated with the FDO device is enough to remove the FDO device
		ov, err := ovs.GetOwnershipVouchersByGUID(guid)
		if err != nil {
			ovs.log.WithFields(logFields).Error("Failed to get ownership voucher by GUID ", guid)
		}
		// Delete the FDO user associated with the FDO device
		fdoUser, err := ovs.GetFDOUserByGUID(guid)
		if err != nil {
			ovs.log.WithFields(logFields).Error("Failed to get FDO user by FDO device ", guid)
		}
		db.DB.Delete(ov)
		db.DB.Delete(fdoUser)
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

// help function to join OwnershipVoucherData & FDOUser with FDODevices
func joinWithFDODevices(guid string) *gorm.DB {
	return db.DB.Joins("JOIN ownership_voucher_data ON ownership_voucher_data.fdo_device_id = fdo_devices.id and ownership_voucher_data.guid = ?",
		guid).Joins("JOIN fdo_users ON fdo_users.fdo_device_id = fdo_devices.id")
}
