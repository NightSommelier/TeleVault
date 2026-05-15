package httpserver

import (
	"net/http"
	"reflect"
	"testing"
)

func TestRedactedHeaderValueHidesSensitiveHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret")
	headers.Set("X-Request-ID", "request-123")

	got := redactedHeaderValue(headers, "Authorization")
	if got != redactedValue {
		t.Fatalf("redactedHeaderValue() = %q, want %q", got, redactedValue)
	}

	got = redactedHeaderValue(headers, "X-Request-ID")
	if got != "request-123" {
		t.Fatalf("redactedHeaderValue() = %q, want request id", got)
	}
}

func TestRedactHeadersCopiesAndRedactsSensitiveValues(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret")
	headers.Set("Cookie", "session=secret")
	headers.Set("X-Request-ID", "request-123")

	got := redactHeaders(headers)

	if !reflect.DeepEqual(got["Authorization"], []string{redactedValue}) {
		t.Fatalf("Authorization = %#v, want redacted", got["Authorization"])
	}
	if !reflect.DeepEqual(got["Cookie"], []string{redactedValue}) {
		t.Fatalf("Cookie = %#v, want redacted", got["Cookie"])
	}
	if !reflect.DeepEqual(got["X-Request-Id"], []string{"request-123"}) {
		t.Fatalf("X-Request-Id = %#v, want copied value", got["X-Request-Id"])
	}

	got["X-Request-Id"][0] = "mutated"
	if headers.Get("X-Request-ID") != "request-123" {
		t.Fatal("redactHeaders returned aliases to original header values")
	}
}

func TestIsSensitiveHeaderIsCaseInsensitive(t *testing.T) {
	tests := []string{
		"authorization",
		"Authorization",
		"AUTHORIZATION",
		"Cookie",
		"X-CSRF-Token",
	}

	for _, tt := range tests {
		if !isSensitiveHeader(tt) {
			t.Fatalf("isSensitiveHeader(%q) = false, want true", tt)
		}
	}
}
