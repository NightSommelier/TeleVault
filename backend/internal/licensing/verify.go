package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const SchemaVersionV1 = 1

type LicenseArtifact struct {
	SchemaVersion int            `json:"schema_version"`
	KeyID         string         `json:"key_id"`
	Payload       LicensePayload `json:"payload"`
	Signature     string         `json:"signature"`
}

type LicensePayload struct {
	LicenseID  string        `json:"license_id"`
	Tier       Tier          `json:"tier"`
	InstanceID string        `json:"instance_id"`
	Limits     LicenseLimits `json:"limits"`
	IssuedAt   time.Time     `json:"issued_at"`
	ExpiresAt  time.Time     `json:"expires_at"`
	GraceDays  int           `json:"grace_days"`
}

type LicenseLimits struct {
	Workspaces                int `json:"workspaces,omitempty"`
	ConnectedTelegramAccounts int `json:"connected_telegram_accounts,omitempty"`
}

type VerifyInput struct {
	RawJSON         []byte
	LocalInstanceID string
	Now             time.Time
	PublicKeys      map[string]ed25519.PublicKey
}

var ErrMissingPublicKeys = errors.New("missing public keys")

func VerifyInstalledLicense(input VerifyInput) (State, error) {
	trimmed := strings.TrimSpace(string(input.RawJSON))
	if trimmed == "" {
		return DefaultState(), nil
	}
	if len(input.PublicKeys) == 0 {
		return State{}, ErrMissingPublicKeys
	}

	now := input.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	invalid := func(code string) State {
		state := State{
			RawLicenseJSON:  trimmed,
			Status:          StatusInvalid,
			Tier:            TierCommunity,
			Limits:          json.RawMessage(`{}`),
			ValidationError: stringPtr(code),
			ValidatedAt:     &now,
		}
		return state.Normalize()
	}

	var artifact LicenseArtifact
	if err := json.Unmarshal([]byte(trimmed), &artifact); err != nil {
		return invalid("invalid_json"), nil
	}
	if artifact.SchemaVersion != SchemaVersionV1 {
		return invalid("unsupported_schema_version"), nil
	}

	tier := normalizeTier(artifact.Payload.Tier)
	state := State{
		RawLicenseJSON: trimmed,
		Status:         StatusInvalid,
		Tier:           tier,
		LicenseID:      stringPtrOrNil(artifact.Payload.LicenseID),
		SchemaVersion:  intPtr(artifact.SchemaVersion),
		KeyID:          stringPtrOrNil(artifact.KeyID),
		InstanceID:     stringPtrOrNil(artifact.Payload.InstanceID),
		IssuedAt:       timePtrOrNil(artifact.Payload.IssuedAt),
		ExpiresAt:      timePtrOrNil(artifact.Payload.ExpiresAt),
		GraceDays:      intPtr(artifact.Payload.GraceDays),
		Limits:         mustMarshalLimits(artifact.Payload.Limits),
		ValidatedAt:    &now,
	}

	if err := validatePayload(artifact.Payload); err != nil {
		state.ValidationError = stringPtr("invalid_payload")
		return state.Normalize(), nil
	}
	if tier != artifact.Payload.Tier {
		state.ValidationError = stringPtr("invalid_tier")
		return state.Normalize(), nil
	}

	publicKey, ok := input.PublicKeys[artifact.KeyID]
	if !ok {
		state.ValidationError = stringPtr("unknown_key_id")
		return state.Normalize(), nil
	}
	if len(publicKey) != ed25519.PublicKeySize {
		state.ValidationError = stringPtr("invalid_public_key")
		return state.Normalize(), nil
	}

	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(artifact.Signature))
	if err != nil || len(signature) != ed25519.SignatureSize {
		state.ValidationError = stringPtr("invalid_signature")
		return state.Normalize(), nil
	}

	payloadBytes, err := json.Marshal(artifact.Payload)
	if err != nil {
		return State{}, err
	}
	if !ed25519.Verify(publicKey, payloadBytes, signature) {
		state.ValidationError = stringPtr("invalid_signature")
		return state.Normalize(), nil
	}

	if input.LocalInstanceID != "" && artifact.Payload.InstanceID != input.LocalInstanceID {
		state.Status = StatusInstanceMismatch
		state.ValidationError = stringPtr("instance_id_mismatch")
		return state.Normalize(), nil
	}

	expiresAt := artifact.Payload.ExpiresAt.UTC()
	graceUntil := expiresAt.Add(time.Duration(artifact.Payload.GraceDays) * 24 * time.Hour)
	if now.After(graceUntil) {
		state.Status = StatusExpired
		state.ValidationError = stringPtr("license_expired")
		return state.Normalize(), nil
	}
	if now.After(expiresAt) {
		state.Status = StatusGrace
		state.ValidationError = stringPtr("license_in_grace")
		return state.Normalize(), nil
	}

	state.Status = StatusValid
	state.ValidationError = nil
	return state.Normalize(), nil
}

func validatePayload(payload LicensePayload) error {
	if strings.TrimSpace(payload.LicenseID) == "" {
		return ErrInvalidState
	}
	if strings.TrimSpace(payload.InstanceID) == "" {
		return ErrInvalidState
	}
	if payload.GraceDays < 0 {
		return ErrInvalidState
	}
	if payload.IssuedAt.IsZero() || payload.ExpiresAt.IsZero() {
		return ErrInvalidState
	}
	if payload.ExpiresAt.Before(payload.IssuedAt) {
		return ErrInvalidState
	}
	if payload.Limits.Workspaces < 0 || payload.Limits.ConnectedTelegramAccounts < 0 {
		return ErrInvalidState
	}
	return nil
}

func normalizeTier(tier Tier) Tier {
	switch tier {
	case TierCommunity, TierPro, TierTeam:
		return tier
	default:
		return TierCommunity
	}
}

func mustMarshalLimits(limits LicenseLimits) json.RawMessage {
	data, err := json.Marshal(limits)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func intPtr(value int) *int {
	return &value
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func timePtrOrNil(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}
