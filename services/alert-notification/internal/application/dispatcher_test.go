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
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/delivery"
	"github.com/greenlab/alert-notification/internal/domain/notification"
)

// --- mock EmailSender ---

type mockEmailSender struct{ mock.Mock }

func (m *mockEmailSender) Send(ctx context.Context, to, subject, body string) error {
	return m.Called(ctx, to, subject, body).Error(0)
}

// --- mock WebhookClient ---

type mockWebhookClient struct{ mock.Mock }

func (m *mockWebhookClient) Post(ctx context.Context, url, payload string) error {
	return m.Called(ctx, url, payload).Error(0)
}

func (m *mockWebhookClient) PostDetailed(ctx context.Context, url, payload, hmacSignature string) (int, string, int64, error) {
	args := m.Called(ctx, url, payload, hmacSignature)
	return args.Int(0), args.String(1), args.Get(2).(int64), args.Error(3)
}

// --- helpers ---

func newTestDispatcher(t *testing.T) (*Dispatcher, *mockEmailSender, *mockWebhookClient, *mockDeliveryRepo) {
	t.Helper()
	email := &mockEmailSender{}
	wh := &mockWebhookClient{}
	dr := &mockDeliveryRepo{}
	d := NewDispatcher(email, wh, dr, slog.Default())
	return d, email, wh, dr
}

// --- tests ---

func TestDispatcher_Email_Success(t *testing.T) {
	d, email, _, _ := newTestDispatcher(t)
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeEmail, "user@example.com", "Alert", "body")

	email.On("Send", mock.Anything, "user@example.com", "Alert", "body").Return(nil)

	err := d.Dispatch(context.Background(), n)
	require.NoError(t, err)
	email.AssertExpectations(t)
}

func TestDispatcher_Email_Error(t *testing.T) {
	d, email, _, _ := newTestDispatcher(t)
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeEmail, "user@example.com", "Alert", "body")
	smtpErr := errors.New("smtp timeout")

	email.On("Send", mock.Anything, "user@example.com", "Alert", "body").Return(smtpErr)

	err := d.Dispatch(context.Background(), n)
	assert.ErrorIs(t, err, smtpErr)
	email.AssertExpectations(t)
}

func TestDispatcher_Webhook_NoSecret_NoSignatureHeader(t *testing.T) {
	d, _, wh, dr := newTestDispatcher(t)
	ruleID := uuid.New()
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeWebhook, "https://example.com/hook", "Alert", `{"v":1}`)
	n.RuleID = &ruleID
	// WebhookSecret is empty — no signature header should be emitted.

	wh.On("PostDetailed", mock.Anything, "https://example.com/hook", `{"v":1}`, "").
		Return(200, "ok", int64(50), nil)
	dr.On("Save", mock.Anything, mock.AnythingOfType("*delivery.Log")).Return(nil)

	err := d.Dispatch(context.Background(), n)
	require.NoError(t, err)
	wh.AssertExpectations(t)
	dr.AssertExpectations(t)
}

func TestDispatcher_Webhook_WithSecret_SignatureHeaderEmitted(t *testing.T) {
	d, _, wh, dr := newTestDispatcher(t)
	ruleID := uuid.New()
	payload := `{"sensor":"temp","value":42}`
	secret := "my-signing-key"
	expectedSig := "sha256=" + alert.ComputeHMAC(secret, payload)

	n := notification.NewNotification(uuid.New(), notification.ChannelTypeWebhook, "https://example.com/hook", "Alert", payload)
	n.RuleID = &ruleID
	n.WebhookSecret = secret

	wh.On("PostDetailed", mock.Anything, "https://example.com/hook", payload, expectedSig).
		Return(200, "ok", int64(50), nil)
	dr.On("Save", mock.Anything, mock.AnythingOfType("*delivery.Log")).Return(nil)

	err := d.Dispatch(context.Background(), n)
	require.NoError(t, err)
	wh.AssertExpectations(t)
	dr.AssertExpectations(t)
}

func TestDispatcher_Webhook_Non2xx_ReturnsError(t *testing.T) {
	d, _, wh, dr := newTestDispatcher(t)
	ruleID := uuid.New()
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeWebhook, "https://example.com/hook", "Alert", `{}`)
	n.RuleID = &ruleID

	wh.On("PostDetailed", mock.Anything, "https://example.com/hook", `{}`, "").
		Return(500, "internal server error", int64(30), nil)
	dr.On("Save", mock.Anything, mock.AnythingOfType("*delivery.Log")).Return(nil)

	err := d.Dispatch(context.Background(), n)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	wh.AssertExpectations(t)
}

func TestDispatcher_Webhook_NetworkError_LoggedAndReturned(t *testing.T) {
	d, _, wh, dr := newTestDispatcher(t)
	ruleID := uuid.New()
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeWebhook, "https://example.com/hook", "Alert", `{}`)
	n.RuleID = &ruleID
	netErr := errors.New("connection refused")

	wh.On("PostDetailed", mock.Anything, "https://example.com/hook", `{}`, "").
		Return(0, "", int64(0), netErr)
	dr.On("Save", mock.Anything, mock.MatchedBy(func(l *delivery.Log) bool {
		return l.ErrorMsg == "connection refused"
	})).Return(nil)

	err := d.Dispatch(context.Background(), n)
	assert.ErrorIs(t, err, netErr)
	wh.AssertExpectations(t)
	dr.AssertExpectations(t)
}

func TestDispatcher_Webhook_NoRuleID_DeliveryLogSkipped(t *testing.T) {
	d, _, wh, dr := newTestDispatcher(t)
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeWebhook, "https://example.com/hook", "Alert", `{}`)
	// n.RuleID is nil — delivery log should NOT be saved

	wh.On("PostDetailed", mock.Anything, "https://example.com/hook", `{}`, "").
		Return(200, "ok", int64(10), nil)

	err := d.Dispatch(context.Background(), n)
	require.NoError(t, err)
	dr.AssertNotCalled(t, "Save")
	wh.AssertExpectations(t)
}

func TestDispatcher_UnsupportedChannelType(t *testing.T) {
	d, _, _, _ := newTestDispatcher(t)
	n := notification.NewNotification(uuid.New(), notification.NotificationChannelType("sms"), "+15550001234", "Alert", "body")

	err := d.Dispatch(context.Background(), n)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported channel type")
}
