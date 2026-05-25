package recovery

import (
	"net/http/httptest"
	"testing"
)

func TestParseImportOptions(t *testing.T) {
	req := httptest.NewRequest("POST", "/recovery/import?mode=replace&confirm_replace=true", nil)
	got := parseImportOptions(req)
	if got.Mode != ImportModeReplace {
		t.Fatalf("parseImportOptions().Mode = %q, want %q", got.Mode, ImportModeReplace)
	}
	if !got.ConfirmReplace {
		t.Fatal("parseImportOptions().ConfirmReplace = false, want true")
	}
}

func TestParseImportOptionsDefaults(t *testing.T) {
	req := httptest.NewRequest("POST", "/recovery/import", nil)
	got := parseImportOptions(req)
	if got.Mode != "" {
		t.Fatalf("parseImportOptions().Mode = %q, want empty for normalization fallback", got.Mode)
	}
	if got.ConfirmReplace {
		t.Fatal("parseImportOptions().ConfirmReplace = true, want false")
	}
}
