// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/services/devicegroups.go

// Package mock_services is a generated GoMock package.
package mock_services

import (
	gomock "github.com/golang/mock/gomock"
	models "github.com/redhatinsights/edge-api/pkg/models"
	gorm "gorm.io/gorm"
	reflect "reflect"
)

// MockDeviceGroupsServiceInterface is a mock of DeviceGroupsServiceInterface interface
type MockDeviceGroupsServiceInterface struct {
	ctrl     *gomock.Controller
	recorder *MockDeviceGroupsServiceInterfaceMockRecorder
}

// MockDeviceGroupsServiceInterfaceMockRecorder is the mock recorder for MockDeviceGroupsServiceInterface
type MockDeviceGroupsServiceInterfaceMockRecorder struct {
	mock *MockDeviceGroupsServiceInterface
}

// NewMockDeviceGroupsServiceInterface creates a new mock instance
func NewMockDeviceGroupsServiceInterface(ctrl *gomock.Controller) *MockDeviceGroupsServiceInterface {
	mock := &MockDeviceGroupsServiceInterface{ctrl: ctrl}
	mock.recorder = &MockDeviceGroupsServiceInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockDeviceGroupsServiceInterface) EXPECT() *MockDeviceGroupsServiceInterfaceMockRecorder {
	return m.recorder
}

// CreateDeviceGroup mocks base method
func (m *MockDeviceGroupsServiceInterface) CreateDeviceGroup(deviceGroup *models.DeviceGroup) (*models.DeviceGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDeviceGroup", deviceGroup)
	ret0, _ := ret[0].(*models.DeviceGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDeviceGroup indicates an expected call of CreateDeviceGroup
func (mr *MockDeviceGroupsServiceInterfaceMockRecorder) CreateDeviceGroup(deviceGroup interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDeviceGroup", reflect.TypeOf((*MockDeviceGroupsServiceInterface)(nil).CreateDeviceGroup), deviceGroup)
}

// GetDeviceGroups mocks base method
func (m *MockDeviceGroupsServiceInterface) GetDeviceGroups(account string, limit, offset int, tx *gorm.DB) (*[]models.DeviceGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeviceGroups", account, limit, offset, tx)
	ret0, _ := ret[0].(*[]models.DeviceGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeviceGroups indicates an expected call of GetDeviceGroups
func (mr *MockDeviceGroupsServiceInterfaceMockRecorder) GetDeviceGroups(account, limit, offset, tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeviceGroups", reflect.TypeOf((*MockDeviceGroupsServiceInterface)(nil).GetDeviceGroups), account, limit, offset, tx)
}

// GetDeviceGroupsCount mocks base method
func (m *MockDeviceGroupsServiceInterface) GetDeviceGroupsCount(account string, tx *gorm.DB) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeviceGroupsCount", account, tx)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeviceGroupsCount indicates an expected call of GetDeviceGroupsCount
func (mr *MockDeviceGroupsServiceInterfaceMockRecorder) GetDeviceGroupsCount(account, tx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeviceGroupsCount", reflect.TypeOf((*MockDeviceGroupsServiceInterface)(nil).GetDeviceGroupsCount), account, tx)
}

// GetDeviceGroupByID mocks base method
func (m *MockDeviceGroupsServiceInterface) GetDeviceGroupByID(ID string) (*models.DeviceGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDeviceGroupByID", ID)
	ret0, _ := ret[0].(*models.DeviceGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDeviceGroupByID indicates an expected call of GetDeviceGroupByID
func (mr *MockDeviceGroupsServiceInterfaceMockRecorder) GetDeviceGroupByID(ID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeviceGroupByID", reflect.TypeOf((*MockDeviceGroupsServiceInterface)(nil).GetDeviceGroupByID), ID)
}

// UpdateDeviceGroup mocks base method
func (m *MockDeviceGroupsServiceInterface) UpdateDeviceGroup(deviceGroup *models.DeviceGroup, account, ID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDeviceGroup", deviceGroup, account, ID)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateDeviceGroup indicates an expected call of UpdateDeviceGroup
func (mr *MockDeviceGroupsServiceInterfaceMockRecorder) UpdateDeviceGroup(deviceGroup, account, ID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDeviceGroup", reflect.TypeOf((*MockDeviceGroupsServiceInterface)(nil).UpdateDeviceGroup), deviceGroup, account, ID)
}
