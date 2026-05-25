package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyInstalledLicenseValid(t *testing.T) {
	keyID := "tv-prod-2026-01"
	publicKey, privateKey := newKeyPair(t)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	payload := testPayload(now)
	raw := signArtifact(t, LicenseArtifact{
		SchemaVersion: SchemaVersionV1,
		KeyID:         keyID,
		Payload:       payload,
	}, privateKey)

	state, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: payload.InstanceID,
		Now:             now,
		PublicKeys: map[string]ed25519.PublicKey{
			keyID: publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense() error = %v", err)
	}

	if state.Status != StatusValid {
		t.Fatalf("status = %q, want %q", state.Status, StatusValid)
	}
	if state.Tier != TierPro {
		t.Fatalf("tier = %q, want %q", state.Tier, TierPro)
	}
	if state.ValidationError != nil {
		t.Fatalf("validation_error = %q, want nil", *state.ValidationError)
	}
}

func TestVerifyInstalledLicenseRejectsInvalidSignature(t *testing.T) {
	keyID := "tv-prod-2026-01"
	publicKey, privateKey := newKeyPair(t)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	payload := testPayload(now)
	raw := signArtifact(t, LicenseArtifact{
		SchemaVersion: SchemaVersionV1,
		KeyID:         keyID,
		Payload:       payload,
	}, privateKey)

	var tampered LicenseArtifact
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	tampered.Payload.ExpiresAt = tampered.Payload.ExpiresAt.Add(24 * time.Hour)
	tamperedRaw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	state, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         tamperedRaw,
		LocalInstanceID: payload.InstanceID,
		Now:             now,
		PublicKeys: map[string]ed25519.PublicKey{
			keyID: publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense() error = %v", err)
	}
	if state.Status != StatusInvalid {
		t.Fatalf("status = %q, want %q", state.Status, StatusInvalid)
	}
	if got := derefString(state.ValidationError); got != "invalid_signature" {
		t.Fatalf("validation_error = %q, want invalid_signature", got)
	}
}

func TestVerifyInstalledLicenseRejectsUnknownKeyID(t *testing.T) {
	_, privateKey := newKeyPair(t)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	payload := testPayload(now)
	raw := signArtifact(t, LicenseArtifact{
		SchemaVersion: SchemaVersionV1,
		KeyID:         "unknown-key",
		Payload:       payload,
	}, privateKey)

	state, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: payload.InstanceID,
		Now:             now,
		PublicKeys:      map[string]ed25519.PublicKey{},
	})
	if err == nil {
		t.Fatalf("VerifyInstalledLicense() error = nil, want ErrMissingPublicKeys")
	}

	publicKey, _ := newKeyPair(t)
	state, err = VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: payload.InstanceID,
		Now:             now,
		PublicKeys: map[string]ed25519.PublicKey{
			"tv-prod-2026-01": publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense() error = %v", err)
	}
	if state.Status != StatusInvalid {
		t.Fatalf("status = %q, want %q", state.Status, StatusInvalid)
	}
	if got := derefString(state.ValidationError); got != "unknown_key_id" {
		t.Fatalf("validation_error = %q, want unknown_key_id", got)
	}
}

func TestVerifyInstalledLicenseInstanceMismatch(t *testing.T) {
	keyID := "tv-prod-2026-01"
	publicKey, privateKey := newKeyPair(t)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	payload := testPayload(now)
	raw := signArtifact(t, LicenseArtifact{
		SchemaVersion: SchemaVersionV1,
		KeyID:         keyID,
		Payload:       payload,
	}, privateKey)

	state, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: "other-instance",
		Now:             now,
		PublicKeys: map[string]ed25519.PublicKey{
			keyID: publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense() error = %v", err)
	}
	if state.Status != StatusInstanceMismatch {
		t.Fatalf("status = %q, want %q", state.Status, StatusInstanceMismatch)
	}
}

func TestVerifyInstalledLicenseGraceAndExpired(t *testing.T) {
	keyID := "tv-prod-2026-01"
	publicKey, privateKey := newKeyPair(t)
	base := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	payload := testPayload(base)
	payload.ExpiresAt = base.Add(-24 * time.Hour)
	payload.GraceDays = 3
	raw := signArtifact(t, LicenseArtifact{
		SchemaVersion: SchemaVersionV1,
		KeyID:         keyID,
		Payload:       payload,
	}, privateKey)

	graceState, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: payload.InstanceID,
		Now:             base,
		PublicKeys: map[string]ed25519.PublicKey{
			keyID: publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense(grace) error = %v", err)
	}
	if graceState.Status != StatusGrace {
		t.Fatalf("grace status = %q, want %q", graceState.Status, StatusGrace)
	}

	expiredState, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: payload.InstanceID,
		Now:             base.Add(5 * 24 * time.Hour),
		PublicKeys: map[string]ed25519.PublicKey{
			keyID: publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense(expired) error = %v", err)
	}
	if expiredState.Status != StatusExpired {
		t.Fatalf("expired status = %q, want %q", expiredState.Status, StatusExpired)
	}
}

func TestVerifyInstalledLicenseRejectsUnsupportedSchema(t *testing.T) {
	keyID := "tv-prod-2026-01"
	publicKey, privateKey := newKeyPair(t)
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	payload := testPayload(now)
	raw := signArtifact(t, LicenseArtifact{
		SchemaVersion: 99,
		KeyID:         keyID,
		Payload:       payload,
	}, privateKey)

	state, err := VerifyInstalledLicense(VerifyInput{
		RawJSON:         raw,
		LocalInstanceID: payload.InstanceID,
		Now:             now,
		PublicKeys: map[string]ed25519.PublicKey{
			keyID: publicKey,
		},
	})
	if err != nil {
		t.Fatalf("VerifyInstalledLicense() error = %v", err)
	}
	if state.Status != StatusInvalid {
		t.Fatalf("status = %q, want %q", state.Status, StatusInvalid)
	}
	if got := derefString(state.ValidationError); got != "unsupported_schema_version" {
		t.Fatalf("validation_error = %q, want unsupported_schema_version", got)
	}
}

func newKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	return publicKey, privateKey
}

func signArtifact(t *testing.T, artifact LicenseArtifact, privateKey ed25519.PrivateKey) []byte {
	t.Helper()
	payloadBytes, err := json.Marshal(artifact.Payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	artifact.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payloadBytes))
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("json.Marshal(artifact) error = %v", err)
	}
	return raw
}

func testPayload(now time.Time) LicensePayload {
	return LicensePayload{
		LicenseID:  "tv_123",
		Tier:       TierPro,
		InstanceID: "instance-1",
		Limits: LicenseLimits{
			Workspaces:                3,
			ConnectedTelegramAccounts: 5,
		},
		IssuedAt:  now.Add(-24 * time.Hour),
		ExpiresAt: now.Add(365 * 24 * time.Hour),
		GraceDays: 30,
	}
}
