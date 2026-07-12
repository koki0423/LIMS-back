package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"IRIS-backend/internal/platform/db"
)

func TestResolveFrontendAssetsDefaultsIndexFile(t *testing.T) {
	distDir := t.TempDir()
	indexPath := filepath.Join(distDir, defaultFrontendIndex)
	if err := os.WriteFile(indexPath, []byte("<html>ok</html>"), 0o600); err != nil {
		t.Fatalf("failed to create index file: %v", err)
	}

	cfg := &db.Config{
		Frontend: db.FrontendConfig{
			Mode:    frontendModeGin,
			DistDir: distDir,
		},
	}

	assets, err := resolveFrontendAssets(cfg)
	if err != nil {
		t.Fatalf("expected frontend assets to resolve, got %v", err)
	}
	if assets.IndexPath != indexPath {
		t.Fatalf("expected default index path %q, got %q", indexPath, assets.IndexPath)
	}
}

func TestResolveFrontendAssetsRequiresDistDirInGinMode(t *testing.T) {
	cfg := &db.Config{
		Frontend: db.FrontendConfig{
			Mode: frontendModeGin,
		},
	}

	if _, err := resolveFrontendAssets(cfg); err == nil {
		t.Fatal("expected gin frontend mode without dist_dir to fail")
	}
}

func TestNewRouterServesFrontendFilesAndSPAFallback(t *testing.T) {
	distDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("failed to create asset dir: %v", err)
	}

	indexContent := "<html><body>spa</body></html>"
	scriptContent := "console.log('spa');"
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(indexContent), 0o600); err != nil {
		t.Fatalf("failed to create index file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "assets", "app.js"), []byte(scriptContent), 0o600); err != nil {
		t.Fatalf("failed to create asset file: %v", err)
	}

	cfg := &db.Config{
		Mode: modeRelease,
		Frontend: db.FrontendConfig{
			Mode:      frontendModeGin,
			DistDir:   distDir,
			IndexFile: "index.html",
		},
	}
	frontendAssets, err := resolveFrontendAssets(cfg)
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
