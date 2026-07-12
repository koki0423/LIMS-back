package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"IRIS-backend/internal/platform/db"
)

func TestResolveFrontendAssetsDefaultsToEmbeddedIndex(t *testing.T) {
	cfg := &db.Config{
		Frontend: db.FrontendConfig{
			Mode: frontendModeGin,
		},
	}

	assets, err := resolveFrontendAssets(cfg)
	if err != nil {
		t.Fatalf("expected frontend assets to resolve, got %v", err)
	}
	if assets.IndexPath != defaultFrontendIndex {
		t.Fatalf("expected default index path %q, got %q", defaultFrontendIndex, assets.IndexPath)
	}
	if !strings.HasPrefix(assets.Source, "embedded:") {
		t.Fatalf("expected embedded frontend source, got %q", assets.Source)
	}
}

func TestResolveFrontendAssetsRejectsMissingEmbeddedIndex(t *testing.T) {
	cfg := &db.Config{
		Frontend: db.FrontendConfig{
			Mode:      frontendModeGin,
			IndexFile: "missing.html",
		},
	}

	if _, err := resolveFrontendAssets(cfg); err == nil {
		t.Fatal("expected gin frontend mode with missing embedded index to fail")
	}
}

func TestNewRouterServesFrontendFilesAndSPAFallback(t *testing.T) {
	indexContent := "<html><body>spa</body></html>"
	scriptContent := "console.log('spa');"
	frontendFS := http.FS(fstest.MapFS{
		"index.html":    {Data: []byte(indexContent)},
		"assets/app.js": {Data: []byte(scriptContent)},
	})

	cfg := &db.Config{
		Mode: modeRelease,
		Frontend: db.FrontendConfig{
			Mode:      frontendModeGin,
			IndexFile: "index.html",
		},
	}
	frontendAssets, err := newFrontendAssets(frontendFS, "index.html", "test")
	if err != nil {
		t.Fatalf("failed to resolve frontend assets: %v", err)
	}

	router := newRouter(modeRelease, nil, cfg, frontendAssets)

	tests := []struct {
		name           string
		path           string
		accept         string
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "root serves index",
			path:           "/",
			accept:         "text/html",
			wantStatusCode: http.StatusOK,
			wantBody:       indexContent,
		},
		{
			name:           "spa route falls back to index",
			path:           "/inventory/dashboard",
			accept:         "text/html",
			wantStatusCode: http.StatusOK,
			wantBody:       indexContent,
		},
		{
			name:           "static asset is served",
			path:           "/assets/app.js",
			accept:         "*/*",
			wantStatusCode: http.StatusOK,
			wantBody:       scriptContent,
		},
		{
			name:           "missing asset stays not found",
			path:           "/assets/missing.js",
			accept:         "*/*",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "unknown api path stays not found",
			path:           "/api/v2/unknown",
			accept:         "text/html",
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("expected status %d, got %d", tt.wantStatusCode, rec.Code)
			}
			if tt.wantBody != "" && strings.TrimSpace(rec.Body.String()) != strings.TrimSpace(tt.wantBody) {
				t.Fatalf("expected body %q, got %q", tt.wantBody, rec.Body.String())
			}
		})
	}
}

func TestLookupFrontendFileReturnsNotFoundForMissingAsset(t *testing.T) {
	frontendFS := http.FS(fstest.MapFS{
		"index.html": {Data: []byte("ok")},
	})

	_, found, err := lookupFrontendFile(frontendFS, "/missing.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected missing asset lookup to report not found")
	}
}

func TestStatFrontendFileTreatsMissingAssetAsNotExist(t *testing.T) {
	frontendFS := http.FS(fstest.MapFS{})

	_, err := statFrontendFile(frontendFS, "missing.js")
	if !isNotExistError(err) {
		t.Fatalf("expected missing asset error to be treated as not-exist, got %v", err)
	}
}

var _ fs.FS = fstest.MapFS{}
