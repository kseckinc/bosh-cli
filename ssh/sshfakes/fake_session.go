// Code generated by counterfeiter. DO NOT EDIT.
package sshfakes

import (
	"sync"

	"github.com/cloudfoundry/bosh-cli/ssh"
)

type FakeSession struct {
	FinishStub        func() error
	finishMutex       sync.RWMutex
	finishArgsForCall []struct {
	}
	finishReturns struct {
		result1 error
	}
	finishReturnsOnCall map[int]struct {
		result1 error
	}
	StartStub        func() (ssh.SSHArgs, error)
	startMutex       sync.RWMutex
	startArgsForCall []struct {
	}
	startReturns struct {
		result1 ssh.SSHArgs
		result2 error
	}
	startReturnsOnCall map[int]struct {
		result1 ssh.SSHArgs
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSession) Finish() error {
	fake.finishMutex.Lock()
	ret, specificReturn := fake.finishReturnsOnCall[len(fake.finishArgsForCall)]
	fake.finishArgsForCall = append(fake.finishArgsForCall, struct {
	}{})
	stub := fake.FinishStub
	fakeReturns := fake.finishReturns
	fake.recordInvocation("Finish", []interface{}{})
	fake.finishMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeSession) FinishCallCount() int {
	fake.finishMutex.RLock()
	defer fake.finishMutex.RUnlock()
	return len(fake.finishArgsForCall)
}

func (fake *FakeSession) FinishCalls(stub func() error) {
	fake.finishMutex.Lock()
	defer fake.finishMutex.Unlock()
	fake.FinishStub = stub
}

func (fake *FakeSession) FinishReturns(result1 error) {
	fake.finishMutex.Lock()
	defer fake.finishMutex.Unlock()
	fake.FinishStub = nil
	fake.finishReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeSession) FinishReturnsOnCall(i int, result1 error) {
	fake.finishMutex.Lock()
	defer fake.finishMutex.Unlock()
	fake.FinishStub = nil
	if fake.finishReturnsOnCall == nil {
		fake.finishReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.finishReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeSession) Start() (ssh.SSHArgs, error) {
	fake.startMutex.Lock()
	ret, specificReturn := fake.startReturnsOnCall[len(fake.startArgsForCall)]
	fake.startArgsForCall = append(fake.startArgsForCall, struct {
	}{})
	stub := fake.StartStub
	fakeReturns := fake.startReturns
	fake.recordInvocation("Start", []interface{}{})
	fake.startMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeSession) StartCallCount() int {
	fake.startMutex.RLock()
	defer fake.startMutex.RUnlock()
	return len(fake.startArgsForCall)
}

func (fake *FakeSession) StartCalls(stub func() (ssh.SSHArgs, error)) {
	fake.startMutex.Lock()
	defer fake.startMutex.Unlock()
	fake.StartStub = stub
}

func (fake *FakeSession) StartReturns(result1 ssh.SSHArgs, result2 error) {
	fake.startMutex.Lock()
	defer fake.startMutex.Unlock()
	fake.StartStub = nil
	fake.startReturns = struct {
		result1 ssh.SSHArgs
		result2 error
	}{result1, result2}
}

func (fake *FakeSession) StartReturnsOnCall(i int, result1 ssh.SSHArgs, result2 error) {
	fake.startMutex.Lock()
	defer fake.startMutex.Unlock()
	fake.StartStub = nil
	if fake.startReturnsOnCall == nil {
		fake.startReturnsOnCall = make(map[int]struct {
			result1 ssh.SSHArgs
			result2 error
		})
	}
	fake.startReturnsOnCall[i] = struct {
		result1 ssh.SSHArgs
		result2 error
	}{result1, result2}
}

func (fake *FakeSession) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.finishMutex.RLock()
	defer fake.finishMutex.RUnlock()
	fake.startMutex.RLock()
	defer fake.startMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeSession) recordInvocation(key string, args []interface{}) {
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

var _ ssh.Session = new(FakeSession)
