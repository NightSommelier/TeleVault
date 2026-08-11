package httpserver

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
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

func TestEmbeddedAppElementIDsExist(t *testing.T) {
	root, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatalf("fs.Sub(static): %v", err)
	}
	indexBytes, err := fs.ReadFile(root, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	appBytes, err := fs.ReadFile(root, "scripts/app.js")
	if err != nil {
		t.Fatalf("read scripts/app.js: %v", err)
	}

	markup := string(indexBytes) + "\n" + string(appBytes)
	app := string(appBytes)
	idPattern := regexp.MustCompile(`\bid=["']([^"']+)["']`)
	ids := make(map[string]struct{})
	for _, match := range idPattern.FindAllStringSubmatch(markup, -1) {
		ids[match[1]] = struct{}{}
	}

	getByIDPattern := regexp.MustCompile(`document\.getElementById\(['"]([^'"]+)['"]\)`)
	seenLookups := make(map[string]struct{})
	for _, match := range getByIDPattern.FindAllStringSubmatch(app, -1) {
		id := match[1]
		if _, checked := seenLookups[id]; checked {
			continue
		}
		seenLookups[id] = struct{}{}
		if _, ok := ids[id]; !ok {
			t.Fatalf("scripts/app.js looks up missing element id %q", id)
		}
	}
	if len(seenLookups) == 0 {
		t.Fatal("scripts/app.js has no document.getElementById lookups; test may be stale")
	}
}

func TestEmbeddedAppExplainsTelegramAppCodeDelivery(t *testing.T) {
	root, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatalf("fs.Sub(static): %v", err)
	}
	appBytes, err := fs.ReadFile(root, "scripts/app.js")
	if err != nil {
		t.Fatalf("read scripts/app.js: %v", err)
	}
	app := string(appBytes)

	for _, want := range []string{
		"Telegram accepted the request and chose app delivery.",
		"Check the Telegram service chat on every already signed-in device; this is not an SMS.",
		"Telegram did not offer an SMS or call fallback for this request.",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("scripts/app.js missing Telegram app delivery hint %q", want)
		}
	}
}

func TestEmbeddedAppShowsTelegramCodeRequestProgress(t *testing.T) {
	root, err := fs.Sub(staticFiles, "static")
	if err != nil {
		t.Fatalf("fs.Sub(static): %v", err)
	}
	indexBytes, err := fs.ReadFile(root, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	appBytes, err := fs.ReadFile(root, "scripts/app.js")
	if err != nil {
		t.Fatalf("read scripts/app.js: %v", err)
	}
	cssBytes, err := fs.ReadFile(root, "css/app.css")
	if err != nil {
		t.Fatalf("read css/app.css: %v", err)
	}
	combined := string(indexBytes) + "\n" + string(appBytes) + "\n" + string(cssBytes)

	for _, want := range []string{
		`id="loginDeliveryState"`,
		"Waiting for Telegram to accept the",
		"Keep this tab open.",
		"Telegram accepted the code request.",
		"Telegram code request failed.",
		"delivery-state.busy::before",
		"aria-busy",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("embedded app missing Telegram code progress marker %q", want)
		}
	}
}
