package licensing

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestStateValidateAcceptsMissingState(t *testing.T) {
	if err := DefaultState().Validate(); err != nil {
		t.Fatalf("DefaultState().Validate() error = %v", err)
	}
}

func TestStateValidateRejectsInvalidShapes(t *testing.T) {
	valid := State{
		RawLicenseJSON: `{"schema_version":1}`,
		Status:         StatusValid,
		Tier:           TierPro,
		Limits:         []byte(`{"workspaces":3}`),
	}

	now := time.Now().UTC()
	keyID := "tv-prod-2026-01"
	licenseID := "tv_123"
	instanceID := "instance-1"
	schemaVersion := 1
	graceDays := 30

	cases := []struct {
		name   string
		mutate func(*State)
	}{
		{
			name: "unknown status",
			mutate: func(state *State) {
				state.Status = "broken"
			},
		},
		{
			name: "unknown tier",
			mutate: func(state *State) {
				state.Tier = "enterprise"
			},
		},
		{
			name: "negative grace days",
			mutate: func(state *State) {
				state.GraceDays = &graceDays
				neg := -1
				state.GraceDays = &neg
			},
		},
		{
			name: "non-positive schema version",
			mutate: func(state *State) {
				zero := 0
				state.SchemaVersion = &zero
			},
		},
		{
			name: "missing raw json for installed state",
			mutate: func(state *State) {
				state.RawLicenseJSON = ""
			},
		},
		{
			name: "missing fields for missing state",
			mutate: func(state *State) {
				state.Status = StatusMissing
				state.Tier = TierPro
			},
		},
		{
			name: "missing state with stray fields",
			mutate: func(state *State) {
				state.Status = StatusMissing
				state.LicenseID = &licenseID
			},
		},
		{
			name: "invalid limits json",
			mutate: func(state *State) {
				state.Limits = []byte(`{invalid`)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := valid
			state.LicenseID = &licenseID
			state.SchemaVersion = &schemaVersion
			state.KeyID = &keyID
			state.InstanceID = &instanceID
			state.IssuedAt = &now
			state.ExpiresAt = &now
			state.GraceDays = &graceDays
			state.ValidationError = stringPtr("example")
			state.InstalledBy = stringPtr("user-1")
			state.InstalledAt = &now
			state.ValidatedAt = &now
			tc.mutate(&state)
			if err := state.Validate(); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("State.Validate() error = %v, want ErrInvalidState", err)
			}
		})
	}
}

func TestStateNormalizeMissingState(t *testing.T) {
	state := State{
		Status:         StatusMissing,
		Tier:           TierTeam,
		RawLicenseJSON: `{"unexpected":true}`,
		Limits:         []byte(`{"workspaces":99}`),
		LicenseID:      stringPtr("tv_1"),
	}

	normalized := state.Normalize()
	if normalized.Status != StatusMissing {
		t.Fatalf("normalized status = %q, want missing", normalized.Status)
	}
	if normalized.Tier != TierCommunity {
		t.Fatalf("normalized tier = %q, want community", normalized.Tier)
	}
	if normalized.RawLicenseJSON != "" || len(normalized.Limits) == 0 {
		t.Fatalf("normalized state retained data unexpectedly: %+v", normalized)
	}
	if normalized.LicenseID != nil || normalized.SchemaVersion != nil || normalized.KeyID != nil || normalized.InstanceID != nil {
		t.Fatalf("normalized state retained license fields unexpectedly: %+v", normalized)
	}
}

func TestScanLicenseState(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 30, 0, 0, time.UTC)
	row := fakeRowScanner{values: []any{
		`{"schema_version":1}`,
		"grace",
		"pro",
		"tv_123",
		int64(1),
		"tv-prod-2026-01",
		"instance-1",
		now,
		now.Add(24 * time.Hour),
		int64(30),
		[]byte(`{"workspaces":3}`),
		"needs renewal",
		"user-1",
		now,
		now.Add(time.Hour),
		now.Add(2 * time.Hour),
	}}

	state, err := scanLicenseState(row)
	if err != nil {
		t.Fatalf("scanLicenseState() error = %v", err)
	}
	if state.Status != StatusGrace || state.Tier != TierPro {
		t.Fatalf("scanLicenseState() status/tier = %q/%q, want grace/pro", state.Status, state.Tier)
	}
	if state.RawLicenseJSON != `{"schema_version":1}` {
		t.Fatalf("scanLicenseState() raw json = %q, want installed payload", state.RawLicenseJSON)
	}
	if got := derefString(state.LicenseID); got != "tv_123" {
		t.Fatalf("scanLicenseState() license_id = %q, want tv_123", got)
	}
	if got := derefInt(state.SchemaVersion); got != 1 {
		t.Fatalf("scanLicenseState() schema_version = %d, want 1", got)
	}
	if got := derefString(state.ValidationError); got != "needs renewal" {
		t.Fatalf("scanLicenseState() validation_error = %q, want needs renewal", got)
	}
	if got := derefString(state.InstalledBy); got != "user-1" {
		t.Fatalf("scanLicenseState() installed_by = %q, want user-1", got)
	}
	if state.UpdatedAt != now.Add(2*time.Hour) {
		t.Fatalf("scanLicenseState() updated_at = %v, want %v", state.UpdatedAt, now.Add(2*time.Hour))
	}
}

type fakeRowScanner struct {
	values []any
}

func (f fakeRowScanner) Scan(dest ...any) error {
	if len(dest) != len(f.values) {
		return fmt.Errorf("scan arity mismatch: got %d dests and %d values", len(dest), len(f.values))
	}
	for i := range dest {
		if err := assignScanValue(dest[i], f.values[i]); err != nil {
			return err
		}
	}
	return nil
}

func assignScanValue(dest any, value any) error {
	switch d := dest.(type) {
	case *sql.NullString:
		if value == nil {
			*d = sql.NullString{}
			return nil
		}
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string for sql.NullString, got %T", value)
		}
		*d = sql.NullString{String: s, Valid: true}
		return nil
	case *sql.NullInt64:
		if value == nil {
			*d = sql.NullInt64{}
			return nil
		}
		n, ok := value.(int64)
		if !ok {
			return fmt.Errorf("expected int64 for sql.NullInt64, got %T", value)
		}
		*d = sql.NullInt64{Int64: n, Valid: true}
		return nil
	case *sql.NullTime:
		if value == nil {
			*d = sql.NullTime{}
			return nil
		}
		tm, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("expected time.Time for sql.NullTime, got %T", value)
		}
		*d = sql.NullTime{Time: tm, Valid: true}
		return nil
	case *time.Time:
		tm, ok := value.(time.Time)
		if !ok {
			return fmt.Errorf("expected time.Time for time.Time, got %T", value)
		}
		*d = tm
		return nil
	case *string:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string for *string, got %T", value)
		}
		*d = s
		return nil
	case *[]byte:
		b, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte for *[]byte, got %T", value)
		}
		*d = append((*d)[:0], b...)
		return nil
	default:
		return fmt.Errorf("unsupported scan destination %T", dest)
	}
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
