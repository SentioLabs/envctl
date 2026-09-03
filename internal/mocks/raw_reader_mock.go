// Hand-written mocks for the optional secrets.RawReader interface.
// The generated MockClient stays untouched; MockRawClient composes it.
package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"
)

// MockRawReader is a testify mock for secrets.RawReader.
type MockRawReader struct {
	mock.Mock
}

// NewMockRawReader creates a MockRawReader that asserts expectations on cleanup.
func NewMockRawReader(t interface {
	mock.TestingT
	Cleanup(func())
},
) *MockRawReader {
	m := &MockRawReader{}
	m.Mock.Test(t) //nolint:staticcheck // QF1008: qualified selector matches mockery's generated style
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

// GetSecretRaw provides a mock function for secrets.RawReader.
func (_m *MockRawReader) GetSecretRaw(ctx context.Context, secretRef string) ([]byte, error) {
	ret := _m.Called(ctx, secretRef)
	if len(ret) == 0 {
		panic("no return value specified for GetSecretRaw")
	}
	var r0 []byte
	if fn, ok := ret.Get(0).(func(context.Context, string) ([]byte, error)); ok {
		return fn(ctx, secretRef)
	}
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]byte)
	}
	return r0, ret.Error(1)
}

// MockRawClient satisfies both secrets.Client and secrets.RawReader by
// forwarding to an embedded MockClient and MockRawReader. Set expectations
// on the Client and Raw fields.
type MockRawClient struct {
	Client *MockClient
	Raw    *MockRawReader
}

// NewMockRawClient creates a MockRawClient whose parts assert expectations on cleanup.
func NewMockRawClient(t interface {
	mock.TestingT
	Cleanup(func())
},
) *MockRawClient {
	return &MockRawClient{Client: NewMockClient(t), Raw: NewMockRawReader(t)}
}

// GetSecret forwards to the embedded MockClient.
func (m *MockRawClient) GetSecret(ctx context.Context, secretRef string) (map[string]string, error) {
	return m.Client.GetSecret(ctx, secretRef)
}

// GetSecretKey forwards to the embedded MockClient.
func (m *MockRawClient) GetSecretKey(ctx context.Context, secretRef, key string) (string, error) {
	return m.Client.GetSecretKey(ctx, secretRef, key)
}

// Name forwards to the embedded MockClient.
func (m *MockRawClient) Name() string {
	return m.Client.Name()
}

// GetSecretRaw forwards to the embedded MockRawReader.
func (m *MockRawClient) GetSecretRaw(ctx context.Context, secretRef string) ([]byte, error) {
	return m.Raw.GetSecretRaw(ctx, secretRef)
}
