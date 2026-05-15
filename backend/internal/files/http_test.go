package files

import "testing"

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
