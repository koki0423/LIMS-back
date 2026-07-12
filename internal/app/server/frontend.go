package server

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"IRIS-backend/internal/platform/db"
)

const (
	frontendModeExternal = "external"
	frontendModeGin      = "gin"
	defaultFrontendIndex = "index.html"
)

type frontendAssets struct {
	DistDir   string
	IndexPath string
}

func validateAppMode(mode string) error {
	if mode != modeDev && mode != modeRelease {
		return fmt.Errorf("unsupported mode %q: expected %s or %s", mode, modeDev, modeRelease)
	}
	return nil
}

func normalizedFrontendMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return frontendModeExternal
	}
	return mode
}

func resolveFrontendAssets(cfg *db.Config) (*frontendAssets, error) {
	mode := normalizedFrontendMode(cfg.Frontend.Mode)

	switch mode {
	case frontendModeExternal:
		return nil, nil
	case frontendModeGin:
	default:
		return nil, fmt.Errorf("unsupported frontend.mode %q: expected %s or %s", cfg.Frontend.Mode, frontendModeExternal, frontendModeGin)
	}

	distDir := strings.TrimSpace(cfg.Frontend.DistDir)
	if distDir == "" {
		return nil, fmt.Errorf("frontend.mode=gin requires frontend.dist_dir")
	}

	resolvedDistDir, err := filepath.Abs(filepath.Clean(distDir))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve frontend.dist_dir: %w", err)
	}
	info, err := os.Stat(resolvedDistDir)
	if err != nil {
		return nil, fmt.Errorf("frontend.dist_dir not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("frontend.dist_dir is not a directory: %s", resolvedDistDir)
	}

	indexFile := strings.TrimSpace(cfg.Frontend.IndexFile)
	if indexFile == "" {
		indexFile = defaultFrontendIndex
	}

	indexPath := filepath.Join(resolvedDistDir, filepath.Clean(indexFile))
	if !pathWithinBase(resolvedDistDir, indexPath) {
		return nil, fmt.Errorf("frontend.index_file must stay within frontend.dist_dir")
	}
	info, err = os.Stat(indexPath)
	if err != nil {
		return nil, fmt.Errorf("frontend.index_file not found: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("frontend.index_file points to a directory: %s", indexPath)
	}

	return &frontendAssets{
		DistDir:   resolvedDistDir,
		IndexPath: indexPath,
	}, nil
}

func shouldEnableDevCORS(mode, frontendMode string) bool {
	return mode == modeDev && normalizedFrontendMode(frontendMode) == frontendModeExternal
}

func registerFrontendRoutes(r *gin.Engine, assets *frontendAssets) {
	if assets == nil {
		return
	}

	serveIndex := func(c *gin.Context) {
		c.File(assets.IndexPath)
	}

	r.GET("/", serveIndex)
	r.HEAD("/", serveIndex)

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		requestPath := c.Request.URL.Path
		if isReservedBackendPath(requestPath) {
			c.Status(http.StatusNotFound)
			return
		}

		filePath, found, err := lookupFrontendFile(assets.DistDir, requestPath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if found {
			c.File(filePath)
			return
		}

		if shouldServeSPAIndex(c.Request, requestPath) {
			serveIndex(c)
			return
		}

		c.Status(http.StatusNotFound)
	})
}

func isReservedBackendPath(requestPath string) bool {
	cleanPath := path.Clean("/" + requestPath)
	return cleanPath == "/api" ||
		strings.HasPrefix(cleanPath, "/api/") ||
		cleanPath == "/swagger" ||
		strings.HasPrefix(cleanPath, "/swagger/")
}

func lookupFrontendFile(distDir, requestPath string) (string, bool, error) {
	cleanPath := path.Clean("/" + requestPath)
	if cleanPath == "/" {
		return "", false, nil
	}

	relativePath := strings.TrimPrefix(cleanPath, "/")
	candidate := filepath.Join(distDir, filepath.FromSlash(relativePath))
	if !pathWithinBase(distDir, candidate) {
		return "", false, fmt.Errorf("requested file escapes dist dir")
	}

	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if info.IsDir() {
		return "", false, nil
	}

	return candidate, true, nil
}

func shouldServeSPAIndex(r *http.Request, requestPath string) bool {
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html") {
		return true
	}
	return path.Ext(requestPath) == ""
}

func pathWithinBase(basePath, targetPath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
