package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/device-registry/internal/domain/field"
	mockdevice "github.com/greenlab/device-registry/internal/mocks/device"
)

// --- mock TxRunner ---

type mockTxRunner struct {
	mock.Mock
}

func (m *mockTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context, tx TxRepos) error) error {
	args := m.Called(ctx, fn)
	// Allow Return(func(ctx, fn) error) so tests can propagate fn's error naturally,
	// making the "RunInTx returns whatever fn returns" contract explicit.
	if f, ok := args.Get(0).(func(context.Context, func(context.Context, TxRepos) error) error); ok {
		return f(ctx, fn)
	}
	return args.Error(0)
}

// --- mock repos for use inside the tx ---

type mockDeviceRepo struct{ mock.Mock }

func (r *mockDeviceRepo) Create(ctx context.Context, d *device.Device) error {
	return r.Called(ctx, d).Error(0)
}
func (r *mockDeviceRepo) GetByID(_ context.Context, _ uuid.UUID) (*device.Device, error) {
	panic("not expected")
}
func (r *mockDeviceRepo) GetByAPIKey(_ context.Context, _ string) (*device.Device, error) {
	panic("not expected")
}
func (r *mockDeviceRepo) ListByWorkspace(_ context.Context, _ uuid.UUID, _, _ int) ([]*device.Device, int64, error) {
	panic("not expected")
}
func (r *mockDeviceRepo) Update(_ context.Context, _ *device.Device) error { panic("not expected") }
func (r *mockDeviceRepo) Delete(_ context.Context, _ uuid.UUID) error      { panic("not expected") }

type mockChannelRepo struct{ mock.Mock }

func (r *mockChannelRepo) Create(ctx context.Context, ch *channel.Channel) error {
	return r.Called(ctx, ch).Error(0)
}
func (r *mockChannelRepo) GetByID(_ context.Context, _ uuid.UUID) (*channel.Channel, error) {
	panic("not expected")
}
func (r *mockChannelRepo) ListByWorkspace(_ context.Context, _ uuid.UUID, _, _ int) ([]*channel.Channel, int64, error) {
	panic("not expected")
}
func (r *mockChannelRepo) ListByDevice(_ context.Context, _ uuid.UUID, _, _ int) ([]*channel.Channel, int64, error) {
	panic("not expected")
}
func (r *mockChannelRepo) Update(_ context.Context, _ *channel.Channel) error { panic("not expected") }
func (r *mockChannelRepo) Delete(_ context.Context, _ uuid.UUID) error        { panic("not expected") }

type mockFieldRepo struct{ mock.Mock }

func (r *mockFieldRepo) Create(ctx context.Context, f *field.Field) error {
	return r.Called(ctx, f).Error(0)
}
func (r *mockFieldRepo) GetByID(_ context.Context, _ uuid.UUID) (*field.Field, error) {
	panic("not expected")
}
func (r *mockFieldRepo) ListByChannel(_ context.Context, _ uuid.UUID) ([]*field.Field, error) {
	panic("not expected")
}
func (r *mockFieldRepo) Update(_ context.Context, _ *field.Field) error { panic("not expected") }
func (r *mockFieldRepo) Delete(_ context.Context, _ uuid.UUID) error    { panic("not expected") }

// --- helpers ---

func validProvisionInput() ProvisionInput {
	return ProvisionInput{
		Device: ProvisionDeviceInput{
			WorkspaceID: uuid.New().String(),
			Name:        "Sensor Node",
			Description: "outdoor sensor",
		},
		Channel: ProvisionChannelInput{
			Name:        "Main Channel",
			Description: "primary data channel",
			Visibility:  "private",
		},
		Fields: []ProvisionFieldInput{
			{Name: "temperature", Label: "Temperature", Unit: "°C", FieldType: "float", Position: 1},
		},
	}
}

// newProvisionSvc is a thin constructor for tests.
func newProvisionSvc(tx *mockTxRunner, cache *mockdevice.MockDeviceCacheRepository) *ProvisionService {
	return NewProvisionService(tx, cache, slog.Default())
}

// --- tests ---

func TestProvision_HappyPath(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	devRepo := &mockDeviceRepo{}
	chRepo := &mockChannelRepo{}
	fRepo := &mockFieldRepo{}

	devRepo.On("Create", ctx, mock.AnythingOfType("*device.Device")).Return(nil)
	chRepo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
	fRepo.On("Create", ctx, mock.AnythingOfType("*field.Field")).Return(nil)

	// Simulate TxRunner calling fn with fake repos.
	txRunner.On("RunInTx", ctx, mock.Anything).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(context.Context, TxRepos) error)
			err := fn(ctx, TxRepos{Devices: devRepo, Channels: chRepo, Fields: fRepo})
			require.NoError(t, err)
		}).
		Return(nil)

	cache.On("SetDevice", ctx, mock.AnythingOfType("*device.Device")).Return(nil)

	result, err := svc.Provision(ctx, validProvisionInput())
	require.NoError(t, err)
	assert.NotEmpty(t, result.Device.APIKey)
	assert.NotNil(t, result.Channel.DeviceID)
	assert.Equal(t, result.Device.ID, *result.Channel.DeviceID)
	assert.Len(t, result.Fields, 1)
	assert.Equal(t, result.Channel.ID, result.Fields[0].ChannelID)

	txRunner.AssertExpectations(t)
	devRepo.AssertExpectations(t)
	chRepo.AssertExpectations(t)
	fRepo.AssertExpectations(t)
}

func TestProvision_RollbackOnFieldCreateFailure(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	devRepo := &mockDeviceRepo{}
	chRepo := &mockChannelRepo{}
	fRepo := &mockFieldRepo{}

	fieldErr := errors.New("unique constraint violation on position")

	devRepo.On("Create", ctx, mock.AnythingOfType("*device.Device")).Return(nil)
	chRepo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
	fRepo.On("Create", ctx, mock.AnythingOfType("*field.Field")).Return(fieldErr)

	// Return a function so the mock propagates fn's error exactly as the real
	// TxRunner would — making the "RunInTx returns whatever fn returns" contract explicit.
	txRunner.On("RunInTx", ctx, mock.Anything).
		Return(func(ctx context.Context, fn func(context.Context, TxRepos) error) error {
			return fn(ctx, TxRepos{Devices: devRepo, Channels: chRepo, Fields: fRepo})
		})

	// Cache must NOT be called — provision failed.
	result, err := svc.Provision(ctx, validProvisionInput())
	assert.Error(t, err)
	assert.Nil(t, result)

	txRunner.AssertExpectations(t)
	devRepo.AssertExpectations(t)
	chRepo.AssertExpectations(t)
	fRepo.AssertExpectations(t)
}

func TestProvision_InvalidWorkspaceID(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	in := validProvisionInput()
	in.Device.WorkspaceID = "not-a-uuid"

	result, err := svc.Provision(ctx, in)
	assert.Error(t, err)
	assert.Nil(t, result)
	// TxRunner must never be called.
	txRunner.AssertNotCalled(t, "RunInTx")
}

func TestProvision_InvalidDeviceName(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	in := validProvisionInput()
	in.Device.Name = ""

	result, err := svc.Provision(ctx, in)
	assert.ErrorIs(t, err, device.ErrInvalidName)
	assert.Nil(t, result)
	txRunner.AssertNotCalled(t, "RunInTx")
}

func TestProvision_InvalidChannelName(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	in := validProvisionInput()
	in.Channel.Name = ""

	result, err := svc.Provision(ctx, in)
	assert.ErrorIs(t, err, channel.ErrInvalidName)
	assert.Nil(t, result)
	txRunner.AssertNotCalled(t, "RunInTx")
}

func TestProvision_InvalidFieldPosition(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	in := validProvisionInput()
	in.Fields[0].Position = 99 // out of range

	result, err := svc.Provision(ctx, in)
	assert.ErrorIs(t, err, field.ErrInvalidPosition)
	assert.Nil(t, result)
	txRunner.AssertNotCalled(t, "RunInTx")
}

func TestProvision_CacheErrorDoesNotFailProvision(t *testing.T) {
	ctx := context.Background()
	txRunner := &mockTxRunner{}
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := newProvisionSvc(txRunner, cache)

	devRepo := &mockDeviceRepo{}
	chRepo := &mockChannelRepo{}
	fRepo := &mockFieldRepo{}

	devRepo.On("Create", ctx, mock.AnythingOfType("*device.Device")).Return(nil)
	chRepo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
	fRepo.On("Create", ctx, mock.AnythingOfType("*field.Field")).Return(nil)

	txRunner.On("RunInTx", ctx, mock.Anything).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(context.Context, TxRepos) error)
			_ = fn(ctx, TxRepos{Devices: devRepo, Channels: chRepo, Fields: fRepo})
		}).
		Return(nil)

	// Cache errors must not propagate.
	cache.On("SetDevice", ctx, mock.AnythingOfType("*device.Device")).Return(errors.New("redis: connection refused"))

	result, err := svc.Provision(ctx, validProvisionInput())
	require.NoError(t, err)
	assert.NotNil(t, result)

	txRunner.AssertExpectations(t)
}
