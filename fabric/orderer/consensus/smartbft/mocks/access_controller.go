// Code generated by mockery v2.14.1. DO NOT EDIT.

package mocks

import (
	protoutil "github.com/hyperledger/fabric/protoutil"
	mock "github.com/stretchr/testify/mock"
)

// AccessController is an autogenerated mock type for the AccessController type
type AccessController struct {
	mock.Mock
}

// Evaluate provides a mock function with given fields: signatureSet
func (_m *AccessController) Evaluate(signatureSet []*protoutil.SignedData) error {
	ret := _m.Called(signatureSet)

	var r0 error
	if rf, ok := ret.Get(0).(func([]*protoutil.SignedData) error); ok {
		r0 = rf(signatureSet)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewAccessController interface {
	mock.TestingT
	Cleanup(func())
}

// NewAccessController creates a new instance of AccessController. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAccessController(t mockConstructorTestingTNewAccessController) *AccessController {
	mock := &AccessController{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
