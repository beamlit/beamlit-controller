// Code generated by MockGen. DO NOT EDIT.
// Source: offloader.go
//
// Generated by this command:
//
//	mockgen -source=offloader.go -destination=offloader_mock.go -package=offloader Offloader
//

// Package offloader is a generated GoMock package.
package offloader

import (
	context "context"
	reflect "reflect"

	v1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	gomock "go.uber.org/mock/gomock"
)

// MockOffloader is a mock of Offloader interface.
type MockOffloader struct {
	ctrl     *gomock.Controller
	recorder *MockOffloaderMockRecorder
}

// MockOffloaderMockRecorder is the mock recorder for MockOffloader.
type MockOffloaderMockRecorder struct {
	mock *MockOffloader
}

// NewMockOffloader creates a new mock instance.
func NewMockOffloader(ctrl *gomock.Controller) *MockOffloader {
	mock := &MockOffloader{ctrl: ctrl}
	mock.recorder = &MockOffloaderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOffloader) EXPECT() *MockOffloaderMockRecorder {
	return m.recorder
}

// Cleanup mocks base method.
func (m *MockOffloader) Cleanup(ctx context.Context, model *v1alpha1.ModelDeployment) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Cleanup", ctx, model)
	ret0, _ := ret[0].(error)
	return ret0
}

// Cleanup indicates an expected call of Cleanup.
func (mr *MockOffloaderMockRecorder) Cleanup(ctx, model any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Cleanup", reflect.TypeOf((*MockOffloader)(nil).Cleanup), ctx, model)
}

// Configure mocks base method.
func (m *MockOffloader) Configure(ctx context.Context, model *v1alpha1.ModelDeployment, backendServiceRef, remoteServiceRef *v1alpha1.ServiceReference, backendWeight int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Configure", ctx, model, backendServiceRef, remoteServiceRef, backendWeight)
	ret0, _ := ret[0].(error)
	return ret0
}

// Configure indicates an expected call of Configure.
func (mr *MockOffloaderMockRecorder) Configure(ctx, model, backendServiceRef, remoteServiceRef, backendWeight any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Configure", reflect.TypeOf((*MockOffloader)(nil).Configure), ctx, model, backendServiceRef, remoteServiceRef, backendWeight)
}