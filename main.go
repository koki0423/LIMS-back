package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"IRIS-backend/internal/asset_mgmt/assets"
	"IRIS-backend/internal/asset_mgmt/disposals"
	"IRIS-backend/internal/asset_mgmt/lends"
	"IRIS-backend/internal/asset_mgmt/printLabels"
	"IRIS-backend/internal/attendance"
	"IRIS-backend/internal/platform/db"
)

// フロントのビルド出力を埋め込む
// "//go:embed public" ← これはビルドに必要なので消さないこと

// go:embed public
var embedded embed.FS

func main() {
	// 設定読み込み
	cfg, err := db.LoadConfig("config/config.yaml")
	if err != nil {
		panic(err)
	}

	// 動作モード取得
	mode := cfg.Mode
	log.Printf("[INFO] mode:%s\n", mode)

	if cfg.Mode != "dev" && cfg.Mode != "release" {
		fmt.Println("Usage: go run main.go [dev|release]")
		return
	}

	conn, err := db.Connect(cfg.DB)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	log.Printf("[INFO] connected to DB: %s", cfg.DB.DBName)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	if mode == "dev" {
		// CORS（開発中のみ必要）
		r.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"http://localhost:3000"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Idempotency-Key"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowCredentials: true,
		}))
	}

	// ヘルス
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// /api/v2
	api := r.Group("/api/v2")
	assets.RegisterRoutes(api, assets.NewService(conn))
	lends.RegisterRoutes(api, lends.NewService(conn))
	disposals.RegisterRoutes(api, disposals.NewService(conn))
	attendance.RegisterRoutes(api, attendance.NewService(conn))
	printLabels.RegisterRoutes(api, printLabels.NewService())

	sub, err := fs.Sub(embedded, "public")
	if err != nil {
		log.Fatal(err)
	}
	fileFS := http.FS(sub)

	r.NoRoute(func(c *gin.Context) {
		// API は対象外
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}

		reqPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if reqPath == "" {
			reqPath = "index.html"
		}

		// 実ファイルがあるならそれを返す（Content-Type を推測、キャッシュ付与）
		if f, err := fileFS.Open(reqPath); err == nil {
			defer f.Close()
			if ct := mime.TypeByExtension(path.Ext(reqPath)); ct != "" {
				c.Header("Content-Type", ct)
			}
			// index.html 以外はキャッシュ（SPAの基本運用）
			if !strings.HasSuffix(reqPath, "index.html") {
				c.Header("Cache-Control", "public, max-age=86400, immutable")
			}
			if fileInfo, err := f.Stat(); err == nil {
				http.ServeContent(c.Writer, c.Request, reqPath, fileInfo.ModTime(), f)
			} else {
				c.Status(http.StatusInternalServerError)
			}
			return
		}

		// なければ index.html にフォールバック
		if idx, err := fileFS.Open("index.html"); err == nil {
			defer idx.Close()
			c.Header("Content-Type", "text/html; charset=utf-8")
			if fileInfo, err := idx.Stat(); err == nil {
				http.ServeContent(c.Writer, c.Request, "index.html", fileInfo.ModTime(), idx)
			} else {
				c.Status(http.StatusInternalServerError)
			}
			return
		}

		c.Status(http.StatusNotFound)
	})

	// TLS起動（:8443 例）
	srv := &http.Server{
		Addr:    ":8443",
		Handler: r,
	}

	var certFile, keyFile string

	// TLS設定
	if mode == "dev" {
		//開発用
		certFile = fmt.Sprintf("config/tls/dev/%s", cfg.Certificate.Cert)
		keyFile = fmt.Sprintf("config/tls/dev/%s", cfg.Certificate.Key)
	} else {
		//本番用
		certFile = fmt.Sprintf("config/tls/release/%s", cfg.Certificate.Cert)
		keyFile = fmt.Sprintf("config/tls/release/%s", cfg.Certificate.Key)
	}

	go func() {
		log.Println("[INFO] listening on https://0.0.0.0:8443")
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("[INFO] shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
