package files

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	password, ok := derivePublicLinkPassword("correct horse battery staple", 8)
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
	if _, ok := derivePublicLinkPassword("short", 8); ok {
		t.Fatal("derivePublicLinkPassword(short) ok = true, want false")
	}
}

func TestFormatPublicFileSizeHumanReadable(t *testing.T) {
	if got := formatPublicFileSize(sql.NullInt64{Int64: 2222981120, Valid: true}); got != "2.1 GB" {
		t.Fatalf("formatPublicFileSize() = %q, want %q", got, "2.1 GB")
	}
}

func TestParseOptionalMaxDownloads(t *testing.T) {
	if got, ok := parseOptionalMaxDownloads(nil); !ok || got.Valid {
		t.Fatalf("parseOptionalMaxDownloads(nil) = %+v,%v, want invalid,true", got, ok)
	}
	v := int64(3)
	if got, ok := parseOptionalMaxDownloads(&v); !ok || !got.Valid || got.Int64 != 3 {
		t.Fatalf("parseOptionalMaxDownloads(3) = %+v,%v, want 3,true", got, ok)
	}
	bad := int64(0)
	if _, ok := parseOptionalMaxDownloads(&bad); ok {
		t.Fatal("parseOptionalMaxDownloads(0) ok = true, want false")
	}
}

func TestParseDownloadLimitMode(t *testing.T) {
	if got, ok := parseDownloadLimitMode(""); !ok || got != PublicDownloadLimitModeHard {
		t.Fatalf("parseDownloadLimitMode(empty) = %q,%v, want hard,true", got, ok)
	}
	if got, ok := parseDownloadLimitMode("soft"); !ok || got != PublicDownloadLimitModeSoft {
		t.Fatalf("parseDownloadLimitMode(soft) = %q,%v, want soft,true", got, ok)
	}
	if _, ok := parseDownloadLimitMode("weird"); ok {
		t.Fatal("parseDownloadLimitMode(weird) ok = true, want false")
	}
}

func TestAcceptsHTML(t *testing.T) {
	r := httptest.NewRequest("GET", "/public/token", nil)
	if !acceptsHTML(r) {
		t.Fatal("acceptsHTML(empty) = false, want true")
	}
	r.Header.Set("Accept", "application/json")
	if acceptsHTML(r) {
		t.Fatal("acceptsHTML(json) = true, want false")
	}
	r.Header.Set("Accept", "application/json, text/html")
	if !acceptsHTML(r) {
		t.Fatal("acceptsHTML(mixed) = false, want true")
	}
}

func TestWritePublicLinkPageWithError(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/public/token", nil)
	file := File{
		NamePlain:      sql.NullString{String: "report.zip", Valid: true},
		PlaintextSize:  sql.NullInt64{Int64: 1024, Valid: true},
		CiphertextSize: sql.NullInt64{Int64: 1200, Valid: true},
	}
	link := PublicLink{PasswordRequired: true}
	h.writePublicLinkPageWithError(w, r, "token", file, link, http.StatusUnauthorized, "Incorrect password. Try again.")

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "Incorrect password. Try again.") {
		t.Fatalf("body does not contain password error message: %s", text)
	}
	if !strings.Contains(text, `action="/public/token/download"`) {
		t.Fatalf("body does not contain download form action: %s", text)
	}
}
