// Server Control 进程入口点。
// 初始化 SQLite (WAL 模式)、设置 chi 路由、挂载所有 API 路由组、
// 启动 HTTP 服务器并支持优雅关停。
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/frp-panel/server-panel/internal/api"
	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/backup"
	"github.com/frp-panel/server-panel/internal/configsync"
	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/frpauth"
	"github.com/frp-panel/server-panel/internal/jobs"
	"github.com/frp-panel/server-panel/internal/operations"
	"github.com/frp-panel/server-panel/internal/routerconfig"
	"github.com/frp-panel/server-panel/internal/session"
	"github.com/frp-panel/server-panel/internal/signing"
	"github.com/frp-panel/server-panel/internal/storage"
	"github.com/frp-panel/server-panel/internal/users"
	"github.com/frp-panel/server-panel/internal/websocket"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// --- 命令行参数 ---
	port := flag.Int("port", 9000, "HTTP server port")
	dataDir := flag.String("data", "", "data directory path (default: ./data in dev, /data in prod)")
	migrationsDir := flag.String("migrations", "", "migrations directory path")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	devMode := flag.Bool("dev", false, "development mode")
	showVer := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVer {
		fmt.Printf("FRP Panel Server Control %s (built: %s)\n", version, buildTime)
		os.Exit(0)
	}

	// --- 日志初始化 ---
	level := slog.LevelInfo
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	logger.Info("starting FRP Panel Server Control",
		"version", version,
		"build_time", buildTime,
	)

	// --- 路径配置 ---
	if *dataDir == "" {
		if *devMode {
			*dataDir = "./data"
		} else {
			*dataDir = "/data"
		}
	}
	if *migrationsDir == "" {
		*migrationsDir = "./migrations"
	}

	// 尝试从可执行文件所在目录推断 migrations 路径
	if _, err := os.Stat(*migrationsDir); os.IsNotExist(err) {
		execPath, _ := os.Executable()
		alt := filepath.Join(filepath.Dir(execPath), "..", "migrations")
		if _, err := os.Stat(alt); err == nil {
			*migrationsDir = alt
		}
	}

	if err := os.MkdirAll(*dataDir, 0o750); err != nil {
		logger.Error("failed to create data directory", "err", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(*dataDir, "panel.db")
	backupDir := filepath.Join(*dataDir, "backups")
	routerSock := filepath.Join(*dataDir, "router.sock")

	// --- 数据库初始化 (WAL, foreign_keys, busy_timeout, synchronous FULL) ---
	dbCfg := storage.DefaultConfig(dbPath)
	db, err := storage.Open(dbCfg)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// 运行迁移
	if err := storage.RunMigrations(db, *migrationsDir, logger); err != nil {
		logger.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	// --- 密钥管理 ---
	encKey := deriveOrCreateKey(db, "enc_key", logger)
	csrfKey := deriveOrCreateKey(db, "csrf_key", logger)
	frpSecret := deriveOrCreateKey(db, "frp_secret", logger)
	routerHMACKey := deriveOrCreateKey(db, "router_hmac", logger)

	// --- 组件初始化 ---
	sessionMgr := session.NewManager(db, logger, session.Config{
		TTL:    24 * time.Hour,
		Secure: !*devMode,
	})

	authn := auth.New(db, logger, sessionMgr, csrfKey)
	auditLog := audit.New(db, logger)
	ops := operations.NewManager(db, logger)
	hub := websocket.NewHub(logger)

	signer := signing.NewSigner(db, logger, encKey)
	activeKey, _ := signer.GetActiveKeyPair()
	if activeKey == nil {
		if _, err := signer.GenerateKeyPair("initial-key"); err != nil {
			logger.Error("failed to generate signing key", "err", err)
			os.Exit(1)
		}
	}

	syncer := configsync.NewSyncer(db, logger, signer)
	routerCfg := routerconfig.NewManager(db, logger, routerHMACKey, routerSock)
	backupMgr := backup.NewManager(db, logger, dbPath, backupDir)
	frpPlugin := frpauth.NewPlugin(db, logger, frpSecret)

	// --- 首次运行: 创建管理员账户 (随机 12 字符密码) ---
	adminUser, adminPass, err := users.InitAdmin(db, logger)
	if err != nil {
		logger.Error("failed to initialize admin user", "err", err)
		os.Exit(1)
	}
	if adminUser != "" {
		logger.Warn("首次运行: 已创建管理员账户",
			"username", adminUser,
			"password", adminPass,
			"important", "请立即登录并修改密码!")
	}

	// --- 后台任务系统 ---
	jobMgr := jobs.NewManager(db, logger, jobs.DefaultConfig())
	jobMgr.Start(context.Background())
	defer jobMgr.Stop()

	// --- WebSocket Hub ---
	hub.Start(context.Background())
	defer hub.Stop()

	// --- 构建路由 ---
	router := api.NewRouter(api.Deps{
		DB:         db,
		Logger:     logger,
		Auth:       authn,
		Session:    sessionMgr,
		Audit:      auditLog,
		Ops:        ops,
		Hub:        hub,
		RouterCfg:  routerCfg,
		ConfigSync: syncer,
		Backup:     backupMgr,
		FrpAuth:    frpPlugin,
		EncKey:     encKey,
		CSRFKey:    csrfKey,
	})

	// --- HTTP 服务器 ---
	addr := fmt.Sprintf(":%d", *port)
	if *devMode {
		addr = fmt.Sprintf(":%d", *port)
	} else {
		addr = fmt.Sprintf("127.0.0.1:%d", *port)
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- 启动 ---
	go func() {
		logger.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	fmt.Println("============================================================")
	fmt.Println("  FRP Panel Server - Control Plane")
	fmt.Println("============================================================")
	fmt.Printf("  Version:  %s\n", version)
	fmt.Printf("  Port:     %d\n", *port)
	fmt.Printf("  Database: %s\n", dbPath)
	fmt.Println("  Admin panel: http://127.0.0.1:9000")
	fmt.Println("  Initial admin credentials shown in logs above.")
	fmt.Println("  You MUST change the password on first login.")
	fmt.Println("============================================================")

	// --- 优雅关停 ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "err", err)
	}
	logger.Info("server exited")
}

// deriveOrCreateKey 从数据库获取或生成一个 32 字节密钥。
func deriveOrCreateKey(db *sql.DB, keyName string, logger *slog.Logger) []byte {
	var stored string
	err := db.QueryRow("SELECT value FROM system_config WHERE key = ?", keyName).Scan(&stored)
	if err == nil {
		key, decErr := crypto.DecodeHex(stored)
		if decErr == nil && len(key) == 32 {
			return key
		}
	}

	key, err := crypto.RandomBytes(32)
	if err != nil {
		logger.Error("failed to generate key", "name", keyName, "err", err)
		os.Exit(1)
	}

	_, err = db.Exec(`
		INSERT OR REPLACE INTO system_config (key, value, updated_at)
		VALUES (?, ?, ?)
	`, keyName, crypto.EncodeHex(key), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		logger.Error("failed to store key", "name", keyName, "err", err)
		os.Exit(1)
	}

	logger.Info("generated new encryption key", "name", keyName)
	return key
}
