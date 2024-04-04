// Code generated by counterfeiter. DO NOT EDIT.
package fake

import (
	"sync"

	"github.com/hyperledger/fabric/common/deliverclient/blocksprovider"
)

type DurationExceededHandler struct {
	DurationExceededHandlerStub        func() bool
	durationExceededHandlerMutex       sync.RWMutex
	durationExceededHandlerArgsForCall []struct {
	}
	durationExceededHandlerReturns struct {
		result1 bool
	}
	durationExceededHandlerReturnsOnCall map[int]struct {
		result1 bool
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *DurationExceededHandler) DurationExceededHandler() bool {
	fake.durationExceededHandlerMutex.Lock()
	ret, specificReturn := fake.durationExceededHandlerReturnsOnCall[len(fake.durationExceededHandlerArgsForCall)]
	fake.durationExceededHandlerArgsForCall = append(fake.durationExceededHandlerArgsForCall, struct {
	}{})
	stub := fake.DurationExceededHandlerStub
	fakeReturns := fake.durationExceededHandlerReturns
	fake.recordInvocation("DurationExceededHandler", []interface{}{})
	fake.durationExceededHandlerMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *DurationExceededHandler) DurationExceededHandlerCallCount() int {
	fake.durationExceededHandlerMutex.RLock()
	defer fake.durationExceededHandlerMutex.RUnlock()
	return len(fake.durationExceededHandlerArgsForCall)
}

func (fake *DurationExceededHandler) DurationExceededHandlerCalls(stub func() bool) {
	fake.durationExceededHandlerMutex.Lock()
	defer fake.durationExceededHandlerMutex.Unlock()
	fake.DurationExceededHandlerStub = stub
}

func (fake *DurationExceededHandler) DurationExceededHandlerReturns(result1 bool) {
	fake.durationExceededHandlerMutex.Lock()
	defer fake.durationExceededHandlerMutex.Unlock()
	fake.DurationExceededHandlerStub = nil
	fake.durationExceededHandlerReturns = struct {
		result1 bool
	}{result1}
}

func (fake *DurationExceededHandler) DurationExceededHandlerReturnsOnCall(i int, result1 bool) {
	fake.durationExceededHandlerMutex.Lock()
	defer fake.durationExceededHandlerMutex.Unlock()
	fake.DurationExceededHandlerStub = nil
	if fake.durationExceededHandlerReturnsOnCall == nil {
		fake.durationExceededHandlerReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.durationExceededHandlerReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *DurationExceededHandler) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.durationExceededHandlerMutex.RLock()
	defer fake.durationExceededHandlerMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *DurationExceededHandler) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ blocksprovider.DurationExceededHandler = new(DurationExceededHandler)
