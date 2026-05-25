package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMountStaticServesIndexAndAssets(t *testing.T) {
	server := &Server{mux: http.NewServeMux()}
	server.mountStatic()

	tests := []struct {
		name      string
		path      string
		contains  string
		cacheHint string
	}{
		{
			name:      "index",
			path:      "/",
			contains:  "/assets/scripts/app.js",
			cacheHint: "no-store",
		},
		{
			name:      "css asset",
			path:      "/assets/css/app.css",
			contains:  ":root",
			cacheHint: "",
		},
		{
			name:      "js asset",
			path:      "/assets/scripts/app.js",
			contains:  "const state",
			cacheHint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			server.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}

			body := rr.Body.String()
			if !strings.Contains(body, tt.contains) {
				t.Fatalf("body does not contain %q", tt.contains)
			}

			if tt.cacheHint != "" {
				cacheControl := rr.Header().Get("Cache-Control")
				if !strings.Contains(cacheControl, tt.cacheHint) {
					t.Fatalf("Cache-Control = %q, want contains %q", cacheControl, tt.cacheHint)
				}
			}
		})
	}
}
