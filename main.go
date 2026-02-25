package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"IRIS-backend/internal/asset_mgmt/assets"
	"IRIS-backend/internal/asset_mgmt/disposals"
	"IRIS-backend/internal/asset_mgmt/lends_new"
	"IRIS-backend/internal/asset_mgmt/printLabels"
	"IRIS-backend/internal/attendance"
	"IRIS-backend/internal/dbmng"
	"IRIS-backend/internal/platform/auth"
	"IRIS-backend/internal/platform/db"
)

const (
	addrListen = "0.0.0.0:8443"

	modeDev     = "dev"
	modeRelease = "release"
)

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
	router := newRouter(cfg.Mode, conn, cfg.Yahoo.AppID)

	// HTTP サーバ生成
	srv := &http.Server{
		Addr:    addrListen,
		Handler: router,
	}

	// TLS の有無を決める（今は一旦 TLS 無効にしておく）
	certFile, keyFile := buildTLSPaths(cfg)
	disableTLSForNow := true
	if disableTLSForNow {
		certFile = ""
		keyFile = ""
	}

	// サーバ起動
	go runServer(srv, certFile, keyFile)

	// Graceful shutdown
	gracefulShutdown(srv, 10*time.Second)
}

// --- 初期化系 ---

func newRouter(mode string, conn *sql.DB, appID string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	if mode == modeDev {
		r.Use(devCORS())
	}

	// ヘルスチェック
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// API ルート登録
	registerAPIRoutes(r, conn, appID)

	return r
}

func devCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{
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

func registerAPIRoutes(r *gin.Engine, conn *sql.DB, appID string) {
	api := r.Group("/api/v2")

	assets.RegisterRoutes(api, assets.NewService(conn, appID))
	lends_new.RegisterRoutes(api, lends_new.NewService(conn))
	disposals.RegisterRoutes(api, disposals.NewService(conn))
	attendance.RegisterRoutes(api, attendance.NewService(conn))
	printLabels.RegisterRoutes(api, printLabels.NewService())
	dbmng.RegisterRoutes(api, dbmng.NewService(conn))
	auth.RegisterRoutes(api, auth.NewService(conn))

	// 管理者用グループ
	admin := api.Group("/admin")
	admin.Use(auth.RequireAuth(auth.JWTSecret()))
	admin.Use(auth.RequireRole("admin"))
	admin.GET("/auth-ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
}

// --- TLS / サーバ起動 ---

func buildTLSPaths(cfg *db.Config) (string, string) {
	var certFile, keyFile string

	if cfg.Mode == modeDev {
		certFile = fmt.Sprintf("config/tls/dev/%s", cfg.Certificate.Cert)
		keyFile = fmt.Sprintf("config/tls/dev/%s", cfg.Certificate.Key)
	} else {
		certFile = fmt.Sprintf("config/tls/release/%s", cfg.Certificate.Cert)
		keyFile = fmt.Sprintf("config/tls/release/%s", cfg.Certificate.Key)
	}

	return certFile, keyFile
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