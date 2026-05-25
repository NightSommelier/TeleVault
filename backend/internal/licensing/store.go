package licensing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidState = errors.New("invalid license state")

type Status string

const (
	StatusMissing          Status = "missing"
	StatusValid            Status = "valid"
	StatusInvalid          Status = "invalid"
	StatusExpired          Status = "expired"
	StatusGrace            Status = "grace"
	StatusInstanceMismatch Status = "instance_mismatch"
)

type Tier string

const (
	TierCommunity Tier = "community"
	TierPro       Tier = "pro"
	TierTeam      Tier = "team"
)

type State struct {
	RawLicenseJSON  string
	Status          Status
	Tier            Tier
	LicenseID       *string
	SchemaVersion   *int
	KeyID           *string
	InstanceID      *string
	IssuedAt        *time.Time
	ExpiresAt       *time.Time
	GraceDays       *int
	Limits          json.RawMessage
	ValidationError *string
	InstalledBy     *string
	InstalledAt     *time.Time
	ValidatedAt     *time.Time
	UpdatedAt       time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func DefaultState() State {
	return State{
		Status: StatusMissing,
		Tier:   TierCommunity,
		Limits: json.RawMessage(`{}`),
	}
}

func (s State) Normalize() State {
	if s.Status == StatusMissing {
		s.RawLicenseJSON = ""
		s.Tier = TierCommunity
		s.LicenseID = nil
		s.SchemaVersion = nil
		s.KeyID = nil
		s.InstanceID = nil
		s.IssuedAt = nil
		s.ExpiresAt = nil
		s.GraceDays = nil
		s.ValidationError = nil
		s.InstalledBy = nil
		s.InstalledAt = nil
		s.ValidatedAt = nil
		s.Limits = json.RawMessage(`{}`)
		return s
	}

	if len(s.Limits) == 0 {
		s.Limits = json.RawMessage(`{}`)
	}

	return s
}

func (s State) Validate() error {
	switch s.Status {
	case StatusMissing, StatusValid, StatusInvalid, StatusExpired, StatusGrace, StatusInstanceMismatch:
	default:
		return ErrInvalidState
	}

	switch s.Tier {
	case TierCommunity, TierPro, TierTeam:
	default:
		return ErrInvalidState
	}

	if s.GraceDays != nil && *s.GraceDays < 0 {
		return ErrInvalidState
	}
	if s.SchemaVersion != nil && *s.SchemaVersion <= 0 {
		return ErrInvalidState
	}
	if len(s.Limits) > 0 && !json.Valid(s.Limits) {
		return ErrInvalidState
	}

	if s.Status == StatusMissing {
		if s.Tier != TierCommunity {
			return ErrInvalidState
		}
		if strings.TrimSpace(s.RawLicenseJSON) != "" {
			return ErrInvalidState
		}
		if s.LicenseID != nil || s.SchemaVersion != nil || s.KeyID != nil || s.InstanceID != nil ||
			s.IssuedAt != nil || s.ExpiresAt != nil || s.GraceDays != nil || s.ValidationError != nil ||
			s.InstalledBy != nil || s.InstalledAt != nil || s.ValidatedAt != nil {
			return ErrInvalidState
		}
		if normalized := strings.TrimSpace(string(s.Limits)); normalized != "" && normalized != "{}" {
			return ErrInvalidState
		}
		return nil
	}

	if strings.TrimSpace(s.RawLicenseJSON) == "" {
		return ErrInvalidState
	}

	return nil
}

func (s *Store) Current(ctx context.Context) (State, error) {
	state, err := scanLicenseState(s.db.QueryRowContext(ctx, `
SELECT raw_license_json, status, tier, license_id, schema_version, key_id, instance_id,
       issued_at, expires_at, grace_days, limits, validation_error, installed_by,
       installed_at, validated_at, updated_at
FROM license_state
WHERE id = TRUE`))
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultState(), nil
	}
	if err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Store) Upsert(ctx context.Context, state State, updatedBy string) (State, error) {
	state = state.Normalize()
	if err := state.Validate(); err != nil {
		return State{}, err
	}

	row := s.db.QueryRowContext(ctx, `
INSERT INTO license_state (
    id, raw_license_json, status, tier, license_id, schema_version, key_id, instance_id,
    issued_at, expires_at, grace_days, limits, validation_error, installed_by, installed_at,
    validated_at, updated_at
)
VALUES (
    TRUE, $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, NULLIF($13, '')::uuid, $14, $15, now()
)
ON CONFLICT (id)
DO UPDATE SET
    raw_license_json = EXCLUDED.raw_license_json,
    status = EXCLUDED.status,
    tier = EXCLUDED.tier,
    license_id = EXCLUDED.license_id,
    schema_version = EXCLUDED.schema_version,
    key_id = EXCLUDED.key_id,
    instance_id = EXCLUDED.instance_id,
    issued_at = EXCLUDED.issued_at,
    expires_at = EXCLUDED.expires_at,
    grace_days = EXCLUDED.grace_days,
    limits = EXCLUDED.limits,
    validation_error = EXCLUDED.validation_error,
    installed_by = EXCLUDED.installed_by,
    installed_at = EXCLUDED.installed_at,
    validated_at = EXCLUDED.validated_at,
    updated_at = now()
RETURNING raw_license_json, status, tier, license_id, schema_version, key_id, instance_id,
          issued_at, expires_at, grace_days, limits, validation_error, installed_by,
          installed_at, validated_at, updated_at`,
		nullableStringParam(state.RawLicenseJSON),
		string(state.Status),
		string(state.Tier),
		nullableStringPtrParam(state.LicenseID),
		nullableIntPtrParam(state.SchemaVersion),
		nullableStringPtrParam(state.KeyID),
		nullableStringPtrParam(state.InstanceID),
		nullableTimePtrParam(state.IssuedAt),
		nullableTimePtrParam(state.ExpiresAt),
		nullableIntPtrParam(state.GraceDays),
		licenseLimitsParam(state.Limits),
		nullableStringPtrParam(state.ValidationError),
		nullableStringParam(updatedBy),
		nullableTimePtrParam(state.InstalledAt),
		nullableTimePtrParam(state.ValidatedAt),
	)
	return scanLicenseState(row)
}

func (s *Store) Clear(ctx context.Context, updatedBy string) (State, error) {
	state := DefaultState()
	return s.Upsert(ctx, state, updatedBy)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLicenseState(row rowScanner) (State, error) {
	var (
		rawLicenseJSON  sql.NullString
		status          string
		tier            string
		licenseID       sql.NullString
		schemaVersion   sql.NullInt64
		keyID           sql.NullString
		instanceID      sql.NullString
		issuedAt        sql.NullTime
		expiresAt       sql.NullTime
		graceDays       sql.NullInt64
		limits          []byte
		validationError sql.NullString
		installedBy     sql.NullString
		installedAt     sql.NullTime
		validatedAt     sql.NullTime
		updatedAt       time.Time
	)

	if err := row.Scan(
		&rawLicenseJSON,
		&status,
		&tier,
		&licenseID,
		&schemaVersion,
		&keyID,
		&instanceID,
		&issuedAt,
		&expiresAt,
		&graceDays,
		&limits,
		&validationError,
		&installedBy,
		&installedAt,
		&validatedAt,
		&updatedAt,
	); err != nil {
		return State{}, err
	}

	state := State{
		Status:    Status(status),
		Tier:      Tier(tier),
		Limits:    append(json.RawMessage(nil), limits...),
		UpdatedAt: updatedAt,
	}
	if rawLicenseJSON.Valid {
		state.RawLicenseJSON = rawLicenseJSON.String
	}
	if licenseID.Valid {
		state.LicenseID = stringPtr(licenseID.String)
	}
	if schemaVersion.Valid {
		value := int(schemaVersion.Int64)
		state.SchemaVersion = &value
	}
	if keyID.Valid {
		state.KeyID = stringPtr(keyID.String)
	}
	if instanceID.Valid {
		state.InstanceID = stringPtr(instanceID.String)
	}
	if issuedAt.Valid {
		state.IssuedAt = timePtr(issuedAt.Time)
	}
	if expiresAt.Valid {
		state.ExpiresAt = timePtr(expiresAt.Time)
	}
	if graceDays.Valid {
		value := int(graceDays.Int64)
		state.GraceDays = &value
	}
	if validationError.Valid {
		state.ValidationError = stringPtr(validationError.String)
	}
	if installedBy.Valid {
		state.InstalledBy = stringPtr(installedBy.String)
	}
	if installedAt.Valid {
		state.InstalledAt = timePtr(installedAt.Time)
	}
	if validatedAt.Valid {
		state.ValidatedAt = timePtr(validatedAt.Time)
	}

	return state, nil
}

func nullableStringParam(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableStringPtrParam(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return *value
}

func nullableIntPtrParam(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTimePtrParam(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func licenseLimitsParam(value json.RawMessage) any {
	if len(value) == 0 {
		return []byte(`{}`)
	}
	return []byte(value)
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	utc := value.UTC()
	return &utc
}
