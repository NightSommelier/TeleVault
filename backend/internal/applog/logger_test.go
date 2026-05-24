package applog

import "testing"

func TestSanitizeComponent(t *testing.T) {
	tests := map[string]string{
		"API":           "api",
		"worker-main":   "worker-main",
		"cleanup_1":     "cleanup_1",
		"***":           "app",
		" migrate run ": "migraterun",
		"":              "app",
	}
	for input, want := range tests {
		got := sanitizeComponent(input)
		if got != want {
			t.Fatalf("sanitizeComponent(%q)=%q want %q", input, got, want)
		}
	}
}
