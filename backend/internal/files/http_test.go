package files

import (
	"database/sql"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := map[string]string{
		" Documents ": "Documents",
		"/bad/name/":  "badname",
		"   ":         "",
	}

	for input, want := range tests {
		if got := normalizeName(input); got != want {
			t.Fatalf("normalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPublicLinkPasswordRoundTrip(t *testing.T) {
	password, ok := derivePublicLinkPassword("correct horse battery staple")
	if !ok {
		t.Fatal("derivePublicLinkPassword() ok = false, want true")
	}

	link := PublicLink{
		PasswordRequired:       true,
		PasswordKDF:            sql.NullString{String: password.KDF, Valid: true},
		PasswordSalt:           password.Salt,
		PasswordHash:           password.Hash,
		PasswordArgonTime:      sql.NullInt64{Int64: int64(password.ArgonTime), Valid: true},
		PasswordArgonMemoryKiB: sql.NullInt64{Int64: int64(password.ArgonMemoryKiB), Valid: true},
		PasswordArgonThreads:   sql.NullInt64{Int64: int64(password.ArgonThreads), Valid: true},
	}

	if !verifyPublicLinkPassword(link, "correct horse battery staple") {
		t.Fatal("verifyPublicLinkPassword(correct) = false, want true")
	}
	if verifyPublicLinkPassword(link, "wrong password") {
		t.Fatal("verifyPublicLinkPassword(wrong) = true, want false")
	}
}

func TestPublicLinkPasswordRejectsShortPasswords(t *testing.T) {
	if _, ok := derivePublicLinkPassword("short"); ok {
		t.Fatal("derivePublicLinkPassword(short) ok = true, want false")
	}
}
