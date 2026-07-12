package server

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"IRIS-backend/internal/platform/db"
)

const (
	frontendModeExternal = "external"
	frontendModeGin      = "gin"
	defaultFrontendIndex = "index.html"
	embeddedFrontendRoot = "web/dist"
)

//go:embed web/dist
var embeddedFrontendFiles embed.FS

type frontendAssets struct {
	FS        http.FileSystem
	IndexPath string
	Source    string
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
		return newEmbeddedFrontendAssets(cfg.Frontend.IndexFile)
	default:
		return nil, fmt.Errorf("unsupported frontend.mode %q: expected %s or %s", cfg.Frontend.Mode, frontendModeExternal, frontendModeGin)
	}
}

func newEmbeddedFrontendAssets(indexFile string) (*frontendAssets, error) {
	subtree, err := fs.Sub(embeddedFrontendFiles, embeddedFrontendRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to open embedded frontend assets: %w", err)
	}

	return newFrontendAssets(http.FS(subtree), indexFile, "embedded:"+embeddedFrontendRoot)
}

func newFrontendAssets(frontendFS http.FileSystem, indexFile, source string) (*frontendAssets, error) {
	indexPath, err := normalizeFrontendAssetPath(indexFile)
	if err != nil {
		return nil, err
	}

	info, err := statFrontendFile(frontendFS, indexPath)
	if err != nil {
		return nil, fmt.Errorf("frontend.index_file not found in %s: %w", source, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("frontend.index_file points to a directory in %s: %s", source, indexPath)
	}

	return &frontendAssets{
		FS:        frontendFS,
		IndexPath: indexPath,
		Source:    source,
	}, nil
}

func normalizeFrontendAssetPath(assetPath string) (string, error) {
	assetPath = strings.TrimSpace(assetPath)
	if assetPath == "" {
		assetPath = defaultFrontendIndex
	}

	cleanPath := path.Clean("/" + assetPath)
	if cleanPath == "/" {
		return "", fmt.Errorf("frontend.index_file must point to a file")
	}

	return strings.TrimPrefix(cleanPath, "/"), nil
}

func statFrontendFile(frontendFS http.FileSystem, assetPath string) (fs.FileInfo, error) {
	file, err := frontendFS.Open(assetPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file.Stat()
}

func shouldEnableDevCORS(mode, frontendMode string) bool {
	return mode == modeDev && normalizedFrontendMode(frontendMode) == frontendModeExternal
}

func registerFrontendRoutes(r *gin.Engine, assets *frontendAssets) {
	if assets == nil {
		return
	}

	serveIndex := func(c *gin.Context) {
		serveFrontendFile(c, assets.FS, assets.IndexPath)
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

		filePath, found, err := lookupFrontendFile(assets.FS, requestPath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if found {
			serveFrontendFile(c, assets.FS, filePath)
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

func lookupFrontendFile(frontendFS http.FileSystem, requestPath string) (string, bool, error) {
	cleanPath := path.Clean("/" + requestPath)
	if cleanPath == "/" {
		return "", false, nil
	}

	relativePath := strings.TrimPrefix(cleanPath, "/")
	info, err := statFrontendFile(frontendFS, relativePath)
	if err != nil {
		if isNotExistError(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if info.IsDir() {
		return "", false, nil
	}

	return relativePath, true, nil
}

func serveFrontendFile(c *gin.Context, frontendFS http.FileSystem, assetPath string) {
	file, err := frontendFS.Open(assetPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}

	if readSeeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), readSeeker)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), bytes.NewReader(data))
}

func shouldServeSPAIndex(r *http.Request, requestPath string) bool {
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html") {
		return true
	}
	return path.Ext(requestPath) == ""
}

func isNotExistError(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
