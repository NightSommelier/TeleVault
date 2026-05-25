package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/licensing"
)

func TestLicensingPersistenceLicenseStateRoundTrip(t *testing.T) {
	database := openIntegrationDB(t)
	store := licensing.NewStore(database)
	ctx := context.Background()

	resetLicenseState(t, database)

	current, err := store.Current(ctx)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.Status != licensing.StatusMissing || current.Tier != licensing.TierCommunity {
		t.Fatalf("Current() = %+v, want missing/community default", current)
	}

	now := time.Date(2026, 5, 25, 12, 30, 0, 0, time.UTC)
	issuedAt := now.Add(-time.Hour)
	expiresAt := now.Add(365 * 24 * time.Hour)
	graceDays := 30
	licenseID := "tv_123"
	keyID := "tv-prod-2026-01"
	instanceID := "instance-1"
	validationError := "needs renewal"
	schemaVersion := 1

	saved, err := store.Upsert(ctx, licensing.State{
		RawLicenseJSON:  `{"schema_version":1,"tier":"pro"}`,
		Status:          licensing.StatusGrace,
		Tier:            licensing.TierPro,
		LicenseID:       &licenseID,
		SchemaVersion:   &schemaVersion,
		KeyID:           &keyID,
		InstanceID:      &instanceID,
		IssuedAt:        &issuedAt,
		ExpiresAt:       &expiresAt,
		GraceDays:       &graceDays,
		Limits:          []byte(`{"workspaces":3,"connected_telegram_accounts":5}`),
		ValidationError: &validationError,
	}, "")
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if saved.Status != licensing.StatusGrace || saved.Tier != licensing.TierPro {
		t.Fatalf("Upsert() = %+v, want grace/pro", saved)
	}
	if saved.RawLicenseJSON != `{"schema_version":1,"tier":"pro"}` {
		t.Fatalf("Upsert() raw json = %q, want installed payload", saved.RawLicenseJSON)
	}
	if got := derefString(saved.LicenseID); got != licenseID {
		t.Fatalf("Upsert() license_id = %q, want %q", got, licenseID)
	}
	if got := derefInt(saved.GraceDays); got != graceDays {
		t.Fatalf("Upsert() grace_days = %d, want %d", got, graceDays)
	}

	current, err = store.Current(ctx)
	if err != nil {
		t.Fatalf("Current() after save error = %v", err)
	}
	if current.Status != licensing.StatusGrace || current.Tier != licensing.TierPro {
		t.Fatalf("Current() after save = %+v, want grace/pro", current)
	}
	if got := derefString(current.KeyID); got != keyID {
		t.Fatalf("Current() key_id = %q, want %q", got, keyID)
	}
	if got := derefString(current.InstanceID); got != instanceID {
		t.Fatalf("Current() instance_id = %q, want %q", got, instanceID)
	}
	if got := derefString(current.ValidationError); got != validationError {
		t.Fatalf("Current() validation_error = %q, want %q", got, validationError)
	}
	if current.UpdatedAt.IsZero() {
		t.Fatal("Current() updated_at = zero, want persisted timestamp")
	}
}

func resetLicenseState(t *testing.T, database *sql.DB) {
	t.Helper()

	reset := func(ctx context.Context) error {
		_, err := database.ExecContext(ctx, `
UPDATE license_state
SET raw_license_json = NULL,
    status = 'missing',
    tier = 'community',
    license_id = NULL,
    schema_version = NULL,
    key_id = NULL,
    instance_id = NULL,
    issued_at = NULL,
    expires_at = NULL,
    grace_days = NULL,
    limits = '{}'::jsonb,
    validation_error = NULL,
    installed_by = NULL,
    installed_at = NULL,
    validated_at = NULL,
    updated_at = now()
WHERE id = TRUE`)
		return err
	}

	if err := reset(context.Background()); err != nil {
		t.Fatalf("reset license state: %v", err)
	}
	t.Cleanup(func() {
		_ = reset(context.Background())
	})
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
