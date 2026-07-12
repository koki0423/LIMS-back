package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"IRIS-backend/internal/asset_mgmt/assets"
	"IRIS-backend/internal/asset_mgmt/computers"
	"IRIS-backend/internal/asset_mgmt/disposals"
	"IRIS-backend/internal/asset_mgmt/lend"
	"IRIS-backend/internal/asset_mgmt/printLabels"
	"IRIS-backend/internal/dbmng"
	"IRIS-backend/internal/platform/auth"
	"IRIS-backend/internal/platform/db"
)

const (
	addrListen = "0.0.0.0:8443"

	modeDev     = "dev"
	modeRelease = "release"
)

func Run(configPath string) error {
	cfg, err := db.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := validateAppMode(cfg.Mode); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	log.Printf("[INFO] mode: %s\n", cfg.Mode)

	frontendAssets, err := resolveFrontendAssets(cfg)
	if err != nil {
		return fmt.Errorf("invalid frontend configuration: %w", err)
	}
	log.Printf("[INFO] frontend mode: %s\n", normalizedFrontendMode(cfg.Frontend.Mode))
	if frontendAssets != nil {
		log.Printf("[INFO] serving frontend from: %s", frontendAssets.Source)
	}

	conn, err := db.Connect(cfg.DB)
	if err != nil {
		return fmt.Errorf("failed to connect DB: %w", err)
	}
	defer conn.Close()
	log.Printf("[INFO] connected to DB: %s", cfg.DB.DBName)

	router := newRouter(cfg.Mode, conn, cfg, frontendAssets)
	srv := &http.Server{
		Addr:    addrListen,
		Handler: router,
	}

	certFile, keyFile, err := resolveServerTLS(cfg)
	if err != nil {
		return fmt.Errorf("failed to resolve TLS configuration: %w", err)
	}

	go runServer(srv, certFile, keyFile)
	gracefulShutdown(srv, 10*time.Second)

	return nil
}

func newRouter(mode string, conn *sql.DB, cfg *db.Config, frontendAssets *frontendAssets) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	if shouldEnableDevCORS(mode, cfg.Frontend.Mode) {
		r.Use(devCORS())
	}

	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	registerAPIRoutes(r, conn, cfg)
	registerFrontendRoutes(r, frontendAssets)

	return r
}

func devCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost",
			"http://127.0.0.1",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Idempotency-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})
}

func registerAPIRoutes(r *gin.Engine, conn *sql.DB, cfg *db.Config) {
	api := r.Group("/api/v2")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	janClient := assets.NewJANClient(cfg.Yahoo.AppID)
	authService := auth.NewService(conn)

	assets.RegisterRoutes(api, assets.NewService(conn, janClient))
	lend.RegisterRoutes(api, lend.NewService(conn))
	disposals.RegisterRoutes(api, disposals.NewService(conn))
	printLabels.RegisterRoutes(api, printLabels.NewService())
	dbmng.RegisterRoutes(api, dbmng.NewService(conn))
	auth.RegisterPublicRoutes(api, authService)

	authenticated := api.Group("")
	authenticated.Use(auth.RequireAuth(auth.JWTSecret(), authService))
	auth.RegisterAuthenticatedRoutes(authenticated, authService)

	computerAdmin := authenticated.Group("")
	computerAdmin.Use(auth.RequireCapability(auth.CapabilityComputersAdmin))
	computers.RegisterRoutes(computerAdmin, computers.NewService(conn))

	assetAdmin := authenticated.Group("")
	assetAdmin.Use(auth.RequireCapability(auth.CapabilityAssetsAdmin))
	auth.RegisterAdminRoutes(assetAdmin, authService)

	admin := authenticated.Group("/admin")
	admin.Use(auth.RequireCapability(auth.CapabilityAssetsAdmin))
	admin.GET("/auth-ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
}

func resolveServerTLS(cfg *db.Config) (string, string, error) {
	if !cfg.TLS {
		return "", "", nil
	}

	return buildTLSPaths(cfg)
}

func buildTLSPaths(cfg *db.Config) (string, string, error) {
	certName := strings.TrimSpace(cfg.Certificate.Cert)
	keyName := strings.TrimSpace(cfg.Certificate.Key)
	if certName == "" || keyName == "" {
		return "", "", fmt.Errorf("tls is enabled but certificate cert/key are not configured")
	}

	baseDir := filepath.Join("config", "tls", cfg.Mode)
	certFile := filepath.Join(baseDir, certName)
	keyFile := filepath.Join(baseDir, keyName)

	if _, err := os.Stat(certFile); err != nil {
		return "", "", fmt.Errorf("certificate file not found: %s", certFile)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return "", "", fmt.Errorf("key file not found: %s", keyFile)
	}

	return certFile, keyFile, nil
}

func runServer(srv *http.Server, certFile, keyFile string) {
	if certFile == "" || keyFile == "" {
		log.Printf("[INFO] no TLS, listening on http://%s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] ListenAndServe: %v", err)
		}
		return
	}

	log.Printf("[INFO] listening on https://%s", srv.Addr)
	if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[FATAL] ListenAndServeTLS: %v", err)
	}
}

func gracefulShutdown(srv *http.Server, timeout time.Duration) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("[INFO] shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[FATAL] server forced to shutdown: %v", err)
	}
}
