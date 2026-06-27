package main

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

	_ "IRIS-backend/docs"
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

// @title           LIMS-back API
// @version         2.0
// @description     This is the API server for the LIMS backend.
// @termsOfService  http://swagger.io/terms/
//
// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io
//
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
//
// @host      localhost:8443
// @BasePath  /api/v2
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
//
// main はアプリケーションのエントリーポイントです。
// Swagger のドキュメンテーションを生成するために、`swag init` コマンドを実行してください。
func main() {
	cfg, err := db.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("[FATAL] failed to load config: %v", err)
	}

	if cfg.Mode != modeDev && cfg.Mode != modeRelease {
		fmt.Println("Usage: go run main.go [dev|release]")
		return
	}
	log.Printf("[INFO] mode: %s\n", cfg.Mode)

	conn, err := db.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("[FATAL] failed to connect DB: %v", err)
	}
	defer conn.Close()
	log.Printf("[INFO] connected to DB: %s", cfg.DB.DBName)

	// Gin ルータ生成（ファイルシステム渡しが不要に）
	router := newRouter(cfg.Mode, conn, cfg)

	// HTTP サーバ生成
	srv := &http.Server{
		Addr:    addrListen,
		Handler: router,
	}

	certFile, keyFile, err := resolveServerTLS(cfg)
	if err != nil {
		log.Fatalf("[FATAL] failed to resolve TLS configuration: %v", err)
	}

	// サーバ起動
	go runServer(srv, certFile, keyFile)

	// Graceful shutdown
	gracefulShutdown(srv, 10*time.Second)
}

// --- 初期化系 ---

func newRouter(mode string, conn *sql.DB, cfg *db.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	if mode == modeDev {
		r.Use(devCORS())
	}

	// ヘルスチェック
	// @Summary Ping server
	// @Description get server health status
	// @Tags health
	// @Accept  json
	// @Produce  plain
	// @Success 200 {string} string "ok"
	// @Router /ping [get]
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// API ルート登録
	registerAPIRoutes(r, conn, cfg)

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

// --- ルーティング ---

func registerAPIRoutes(r *gin.Engine, conn *sql.DB, cfg *db.Config) {
	api := r.Group("/api/v2")
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	janClient := assets.NewJANClient(cfg.Yahoo.AppID)

	assets.RegisterRoutes(api, assets.NewService(conn, janClient))
	computers.RegisterRoutes(api, computers.NewService(conn))
	lend.RegisterRoutes(api, lend.NewService(conn))
	disposals.RegisterRoutes(api, disposals.NewService(conn))
	printLabels.RegisterRoutes(api, printLabels.NewService())
	dbmng.RegisterRoutes(api, dbmng.NewService(conn))
	auth.RegisterRoutes(api, auth.NewService(conn))

	// 管理者用グループ
	admin := api.Group("/admin")
	admin.Use(auth.RequireAuth(auth.JWTSecret()))
	admin.Use(auth.RequireRole("admin"))
	// @Summary Ping server with authentication
	// @Description get server health status (requires admin role)
	// @Tags health,admin
	// @Accept  json
	// @Produce  plain
	// @Success 200 {string} string "ok"
	// @Security BearerAuth
	// @Router /admin/auth-ping [get]
	admin.GET("/auth-ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
}

// --- TLS / サーバ起動 ---

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

// --- Graceful shutdown ---

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
