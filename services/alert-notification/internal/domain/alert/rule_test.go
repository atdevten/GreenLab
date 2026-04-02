package alert

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHashSecret(t *testing.T) {
	h1 := HashSecret("mysecret")
	h2 := HashSecret("mysecret")
	h3 := HashSecret("other")

	assert.Len(t, h1, 64, "SHA-256 hex digest is 64 chars")
	assert.Equal(t, h1, h2, "same input produces same hash")
	assert.NotEqual(t, h1, h3, "different inputs produce different hashes")
}

func TestComputeHMAC(t *testing.T) {
	sig := ComputeHMAC("secret", `{"field":"value"}`)
	assert.Len(t, sig, 64, "HMAC-SHA256 hex is 64 chars")
	assert.Equal(t, sig, ComputeHMAC("secret", `{"field":"value"}`), "deterministic")
	assert.NotEqual(t, sig, ComputeHMAC("other", `{"field":"value"}`), "different key differs")
	assert.NotEqual(t, sig, ComputeHMAC("secret", `{"field":"other"}`), "different payload differs")
}

func TestValidateHMAC(t *testing.T) {
	secret := "my-signing-key"
	payload := `{"sensor":"temp","value":42}`
	mac := ComputeHMAC(secret, payload)
	sig := "sha256=" + mac

	tests := []struct {
		name      string
		secret    string
		payload   string
		signature string
		want      bool
	}{
		{"valid signature", secret, payload, sig, true},
		{"wrong secret", "wrong-key", payload, sig, false},
		{"wrong payload", secret, `{"sensor":"temp","value":99}`, sig, false},
		{"wrong prefix", secret, payload, "hmac=" + mac, false},
		{"empty signature", secret, payload, "", false},
		{"empty secret", "", payload, sig, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateHMAC(tc.secret, tc.payload, tc.signature)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewRule(t *testing.T) {
	chID := uuid.New()
	wsID := uuid.New()

	r := NewRule(chID, wsID, "high temp", "temperature", ConditionGT, 80.0, SeverityWarning)

	assert.Equal(t, chID, r.ChannelID)
	assert.Equal(t, wsID, r.WorkspaceID)
	assert.Equal(t, "high temp", r.Name)
	assert.Equal(t, "temperature", r.FieldName)
	assert.Equal(t, ConditionGT, r.Condition)
	assert.Equal(t, 80.0, r.Threshold)
	assert.Equal(t, SeverityWarning, r.Severity)
	assert.True(t, r.Enabled)
	assert.Equal(t, 300, r.CooldownSec)
	assert.False(t, r.ID == uuid.Nil)
	assert.False(t, r.CreatedAt.IsZero())
}

func TestEvaluate(t *testing.T) {
	base := NewRule(uuid.New(), uuid.New(), "r", "f", ConditionGT, 50.0, SeverityInfo)

	tests := []struct {
		name      string
		condition RuleCondition
		threshold float64
		value     float64
		want      bool
	}{
		{"gt true", ConditionGT, 50, 51, true},
		{"gt false equal", ConditionGT, 50, 50, false},
		{"gt false below", ConditionGT, 50, 49, false},
		{"gte true equal", ConditionGTE, 50, 50, true},
		{"gte true above", ConditionGTE, 50, 51, true},
		{"gte false below", ConditionGTE, 50, 49, false},
		{"lt true", ConditionLT, 50, 49, true},
		{"lt false equal", ConditionLT, 50, 50, false},
		{"lte true equal", ConditionLTE, 50, 50, true},
		{"lte true below", ConditionLTE, 50, 49, true},
		{"lte false above", ConditionLTE, 50, 51, false},
		{"eq true", ConditionEQ, 50, 50, true},
		{"eq false", ConditionEQ, 50, 51, false},
		{"neq true", ConditionNEQ, 50, 51, true},
		{"neq false equal", ConditionNEQ, 50, 50, false},
		{"threshold zero neq", ConditionNEQ, 0, 1, true},
		{"threshold zero eq", ConditionEQ, 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := *base
			r.Condition = tc.condition
			r.Threshold = tc.threshold
			assert.Equal(t, tc.want, r.Evaluate(tc.value))
		})
	}
}

func TestEvaluate_UnknownCondition(t *testing.T) {
	r := NewRule(uuid.New(), uuid.New(), "r", "f", RuleCondition("unknown"), 10, SeverityInfo)
	assert.False(t, r.Evaluate(100))
}
