//go:build fdo
// +build fdo

package services

import (
	"context"
	"errors"

	libfdo "github.com/fedora-iot/fido-device-onboard-rs/libfdo-data-go"
	"github.com/redhatinsights/edge-api/pkg/clients/fdo"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// OwnershipVoucherService for ownership voucher management
type OwnershipVoucherService struct {
	Service
}

// OwnershipVoucherServiceInterface is the interface for the ownership voucher service
type OwnershipVoucherServiceInterface interface {
	BatchUploadOwnershipVouchers(voucherBytes []byte, numOfOVs uint) (interface{}, error)
	BatchDeleteOwnershipVouchers(fdoUUIDList []string) (interface{}, error)
	ConnectDevices(fdoUUIDList []string) ([]interface{}, []error)
	ParseOwnershipVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error)
	GetFDODeviceByGUID(ownershipVoucherGUID string) (*models.FDODevice, error)
	storeFDODevices(data []models.OwnershipVoucherData)
	removeFDODevices(fdoUUIDList []string)
	parseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error)
	createFDOClient() *fdo.Client
}

// NewOwnershipVoucherService creates a new ownership voucher service
func NewOwnershipVoucherService(ctx context.Context, log log.FieldLogger) OwnershipVoucherServiceInterface {
	return &OwnershipVoucherService{
		Service: Service{ctx: ctx, log: log.WithField("service", "ownershipvoucher")},
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
			ovs.log.WithFields(logFields).WithFields(log.Fields{"guid": guid, "error": err}).Warn("Couldn't find OwnershipVoucher")
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
	result := preloadFDODevices(ownershipVoucherGUID).First(&fdoDevice)
	if result.Error != nil {
		ovs.log.WithFields(logFields).WithField("error", result.Error).Error("Failed to get FDO device by GUID")
		return nil, result.Error
	}
	return &fdoDevice, nil
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
		result := preloadFDODevices(voucherData.GUID).FirstOrCreate(&fdoDevice)
		if result.Error != nil {
			ovs.log.WithFields(logFields).WithField("error", result.Error).Error("Failed to store FDO device")
		}
	}
}

// removeFDODevices removes FDO devices from the database
func (ovs *OwnershipVoucherService) removeFDODevices(fdoUUIDList []string) {
	logFields := log.Fields{"method": "services.removeFDODevices"}
	for _, guid := range fdoUUIDList {
		var fdoDevice models.FDODevice
		result := preloadFDODevices(guid).First(&fdoDevice)
		if result.Error != nil {
			ovs.log.WithFields(logFields).WithField("error", result.Error).Error("Failed to remove FDO device")
		}
		db.DB.Delete(&fdoDevice)
	}
}

// parseVouchers parses vouchers from a byte array, returning the data and error if any
func (ovs *OwnershipVoucherService) parseVouchers(voucherBytes []byte) ([]models.OwnershipVoucherData, error) {
	logFields := log.Fields{"method": "services.parseVouchers"}
	vouchers, err := libfdo.ParseManyOwnershipVouchers(voucherBytes)
	if err != nil {
		ovs.log.WithFields(logFields).WithField("error", err).Error("Failed to parse vouchers")
		return nil, err
	}
	defer vouchers.Free()

	data := make([]models.OwnershipVoucherData, vouchers.Len())
	for i := 0; i < vouchers.Len(); i++ {
		voucher, err := vouchers.GetVoucher(i)
		if err != nil {
			ovs.log.WithFields(logFields).WithField("error", err).Error("Failed to get voucher")
			return nil, err
		}
		data[i] = models.OwnershipVoucherData{
			GUID:            voucher.GetGUID(),
			ProtocolVersion: voucher.GetProtocolVersion(),
			DeviceName:      voucher.GetDeviceInfo(),
		}
	}
	return data, nil
}

// createFDOClient creates a new FDO client
func (ovs *OwnershipVoucherService) createFDOClient() *fdo.Client {
	return fdo.InitClient(ovs.ctx, ovs.log)
}

// help function to Join Preload association data using inner join
// this will load OwnershipVoucherData & FDOUser with FDODevices
func preloadFDODevices(guid string) *gorm.DB {
	return db.DB.Joins("OwnershipVoucherData").Joins("InitialUser").Find(&models.FDODevice{},
		"guid = ?", guid)
}
