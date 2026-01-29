package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"

	"IRIS-backend/internal/asset_mgmt/assets"
	"IRIS-backend/internal/asset_mgmt/disposals"
	// "IRIS-backend/internal/asset_mgmt/lends"
	"IRIS-backend/internal/asset_mgmt/lends_new"
	"IRIS-backend/internal/asset_mgmt/printLabels"
	"IRIS-backend/internal/attendance"
	"IRIS-backend/internal/dbmng"
	"IRIS-backend/internal/platform/auth"
	"IRIS-backend/internal/platform/db"
)

// --- フロントエンド配信ソース（埋め込み or ディレクトリ）---
//
// 目的:
// - go build . : 埋め込み配信（単一バイナリ）
// - go run .   : 指定ディレクトリ配信（フロントを外部でビルドして差し替えやすく）
//
// 切替:
// - 環境変数 FRONTEND_MODE があれば最優先（embed | dir）
// - それ以外は「go run 実行っぽい」なら dir、そうでなければ embed
//
// ディレクトリ指定（dir のとき）:
// - FRONTEND_APP1_DIR（例: ../IRIS-frontend/app1/dist）
// - FRONTEND_APP2_DIR（例: ../IRIS-frontend/app2/dist）
//
// ※ 埋め込み配信のとき TLS 化しないと WebUSB が動かないので注意（localhost は例外）
//
// //go:embed app1/** app2/**
var embeddedUI embed.FS

const (
	addrListen = "0.0.0.0:8443"

	modeDev     = "dev"
	modeRelease = "release"

	frontendModeEmbed = "embed"
	frontendModeDir   = "dir"
)

func init() {
	// OS/環境によっては .js の MIME が空になるケース対策（念のため）
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".mjs", "application/javascript")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
}

func main() {
	cfg, err := db.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("[FATAL] failed to load config: %v", err)
	}

	if cfg.Mode != modeDev && cfg.Mode != modeRelease {
		fmt.Println("Usage: set mode=dev|release in config/config.yaml")
		return
	}
	log.Printf("[INFO] mode: %s", cfg.Mode)

	conn, err := db.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("[FATAL] failed to connect DB: %v", err)
	}
	defer conn.Close()
	log.Printf("[INFO] connected to DB: %s", cfg.DB.DBName)

	// --- フロント配信用 FileSystem を決定（embed / dir 切替）---
	source := resolveFrontendSource()

	var fileFS http.FileSystem
	var fileFS2 http.FileSystem
	switch source.Mode {
	case frontendModeDir:
		fileFS = mustDirFS(source.App1Dir)
		fileFS2 = mustDirFS(source.App2Dir)
		log.Printf("[INFO] frontend: %s (app1=%s, app2=%s)", source.Mode, source.App1Dir, source.App2Dir)
	default:
		// embed
		fileFS = mustSubFS(embeddedUI, "app1")
		fileFS2 = mustSubFS(embeddedUI, "app2")
		log.Printf("[INFO] frontend: %s", frontendModeEmbed)
	}

	// Gin ルータ生成
	router := newRouter(cfg.Mode, conn, fileFS, fileFS2)

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

type frontendSource struct {
	Mode    string
	App1Dir string
	App2Dir string
}

func resolveFrontendSource() frontendSource {
	// 明示指定があればそれが最優先
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("FRONTEND_MODE"))); v != "" {
		if v == frontendModeDir || v == frontendModeEmbed {
			app1 := strings.TrimSpace(os.Getenv("FRONTEND_APP1_DIR"))
			app2 := strings.TrimSpace(os.Getenv("FRONTEND_APP2_DIR"))
			if app1 == "" {
				app1 = "app1"
			}
			if app2 == "" {
				app2 = "app2"
			}
			return frontendSource{Mode: v, App1Dir: app1, App2Dir: app2}
		}
		log.Printf("[WARN] invalid FRONTEND_MODE=%q (use embed|dir). fallback to auto", v)
	}

	// go run っぽいなら dir、それ以外は embed
	if looksLikeGoRun() {
		app1 := strings.TrimSpace(os.Getenv("FRONTEND_APP1_DIR"))
		app2 := strings.TrimSpace(os.Getenv("FRONTEND_APP2_DIR"))
		if app1 == "" {
			app1 = "app1"
		}
		if app2 == "" {
			app2 = "app2"
		}
		return frontendSource{Mode: frontendModeDir, App1Dir: app1, App2Dir: app2}
	}

	return frontendSource{Mode: frontendModeEmbed}
}

func looksLikeGoRun() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	p := strings.ToLower(exe)

	// go run は一時ディレクトリに go-build... みたいなパスを作ることが多い（Win/Unix どちらも）
	if strings.Contains(p, "go-build") {
		return true
	}
	// 念のため（環境によっては cache パスになることがある）
	if strings.Contains(p, string(filepath.Separator)+"go-build") {
		return true
	}
	return false
}

// --- 初期化系 ---

func mustSubFS(efs embed.FS, dir string) http.FileSystem {
	sub, err := fs.Sub(efs, dir)
	if err != nil {
		log.Fatalf("[FATAL] failed to create sub filesystem: %v", err)
	}
	return http.FS(sub)
}

func mustDirFS(dir string) http.FileSystem {
	// 未指定ならリポジトリ直下の app1/app2 を見る（開発で置きたいならここに dist をコピーしてもOK）
	if strings.TrimSpace(dir) == "" {
		// 呼び出し側で app1/app2 を使い分けたいので、ここでは空のまま返さない
		// 具体的なデフォルトは呼び出し側で渡すことを想定
		log.Fatalf("[FATAL] frontend dir is empty (set FRONTEND_APP1_DIR / FRONTEND_APP2_DIR)")
	}

	if _, err := os.Stat(dir); err != nil {
		log.Fatalf("[FATAL] frontend dir not found: %s (%v)", dir, err)
	}
	return http.FS(os.DirFS(dir))
}

func newRouter(mode string, conn *sql.DB, fileFS http.FileSystem, fileFS2 http.FileSystem) *gin.Engine {
	if mode == modeDev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	if mode == modeDev {
		r.Use(devCORS())
	}

	// ヘルスチェック
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// API ルート登録
	registerAPIRoutes(r, conn)

	// SPA 配信（API 以外の全てをフロントに流す）
	registerSPARoutes(r, fileFS, fileFS2)

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

func registerAPIRoutes(r *gin.Engine, conn *sql.DB) {
	api := r.Group("/api/v2")

	assets.RegisterRoutes(api, assets.NewService(conn))
	// lends.RegisterRoutes(api, lends.NewService(conn))
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

func registerSPARoutes(r *gin.Engine, fileFS http.FileSystem, fileFS2 http.FileSystem) {
	// 入口（好きに変更してOK）
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/app1/") })

	r.GET("/app1/*filepath", func(c *gin.Context) {
		serveSPA(c, fileFS)
	})
	r.GET("/app1", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/app1/") })

	r.GET("/app2/*filepath", func(c *gin.Context) {
		serveSPA(c, fileFS2)
	})
	r.GET("/app2", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/app2/") })

	// その他は 404（必要ならここで何か別のハンドリング）
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusNotFound)
	})
}

// SPA 配信
func serveSPA(c *gin.Context, fileFS http.FileSystem) {
	reqPath := strings.TrimPrefix(c.Param("filepath"), "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	// 実ファイルがあるならそれを返す（Content-Type を推測、キャッシュ付与）
	f, err := fileFS.Open(reqPath)
	if err == nil {
		defer f.Close()

		if ct := mime.TypeByExtension(path.Ext(reqPath)); ct != "" {
			c.Header("Content-Type", ct)
		}

		// index.html 以外はキャッシュ
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

	ext := path.Ext(reqPath)
	if ext != "" {
		// .js/.css/.wasm など “静的ファイルっぽい” のに無いなら 404 を返す
		c.Status(http.StatusNotFound)
		return
	}

	// index.html にフォールバック
	idx, err := fileFS.Open("index.html")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer idx.Close()

	c.Header("Content-Type", "text/html; charset=utf-8")
	if fileInfo, err := idx.Stat(); err == nil {
		http.ServeContent(c.Writer, c.Request, "index.html", fileInfo.ModTime(), idx)
	} else {
		c.Status(http.StatusInternalServerError)
	}
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
