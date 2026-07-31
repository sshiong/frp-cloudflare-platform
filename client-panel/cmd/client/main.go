package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/frp-panel/client-panel/internal/activesession"
	"github.com/frp-panel/client-panel/internal/configapply"
	"github.com/frp-panel/client-panel/internal/frpc"
	"github.com/frp-panel/client-panel/internal/hmacsigner"
	"github.com/frp-panel/client-panel/internal/localapi"
	"github.com/frp-panel/client-panel/internal/logs"
	"github.com/frp-panel/client-panel/internal/securestore"
	"github.com/frp-panel/client-panel/internal/serverbinding"
	"github.com/frp-panel/client-panel/internal/serverclient"
	"github.com/frp-panel/client-panel/internal/sessionproxy"
	"github.com/frp-panel/client-panel/internal/storage"
	"github.com/frp-panel/client-panel/internal/supervisor"
	wsclient "github.com/frp-panel/client-panel/internal/websocket"
)

const (
	// Version Client Panel 版本
	Version = "0.1.0"

	// DefaultListenAddr 默认监听地址
	DefaultListenAddr = "127.0.0.1:7410"

	// FRPCBinaryName FRPC 二进制名称
	FRPCBinaryName = "frpc"
)

func main() {
	// 命令行参数
	var (
		dataDir    string
		listenAddr string
		logLevel   string
		showVer    bool
	)

	flag.StringVar(&dataDir, "data-dir", defaultDataDir(), "数据目录")
	flag.StringVar(&listenAddr, "listen", DefaultListenAddr, "监听地址")
	flag.StringVar(&logLevel, "log-level", "info", "日志级别 (debug, info, warn, error)")
	flag.BoolVar(&showVer, "version", false, "显示版本")
	flag.Parse()

	if showVer {
		fmt.Printf("FRP Client Panel v%s\n", Version)
		os.Exit(0)
	}

	// 初始化日志管理器
	logDir := filepath.Join(dataDir, "logs")
	logMgr, err := logs.NewManager(logs.Config{
		LogDir:   logDir,
		Level:    logLevel,
		MaxSize:  50 * 1024 * 1024, // 50MB
		MaxFiles: 10,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logMgr.Close()

	logger := logMgr.Logger()
	logger.Info("FRP Client Panel 启动中",
		"version", Version,
		"data_dir", dataDir,
		"listen", listenAddr,
		"pid", os.Getpid(),
		"runtime", runtime.GOOS+"/"+runtime.GOARCH,
	)

	// 创建根 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 文件锁/互斥量：确保单实例运行
	unlock, err := acquireFileLock(dataDir)
	if err != nil {
		logger.Error("获取文件锁失败（可能已有实例在运行）", "error", err)
		os.Exit(1)
	}
	defer unlock()

	// 初始化本地 SQLite 数据库
	dbPath := filepath.Join(dataDir, "client.db")
	db, err := storage.Open(ctx, dbPath, logger)
	if err != nil {
		logger.Error("打开数据库失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 检查并生成 installation_instance_id
	install, err := db.GetInstallation(ctx)
	if err != nil {
		logger.Error("读取安装实例失败", "error", err)
		os.Exit(1)
	}

	if install.InstallationInstanceID == "" {
		newID := uuid.New().String()
		if err := db.SetInstallationID(ctx, newID); err != nil {
			logger.Error("设置安装实例 ID 失败", "error", err)
			os.Exit(1)
		}
		logger.Info("生成新的安装实例 ID", "id", newID)
		install.InstallationInstanceID = newID
	} else {
		logger.Info("使用现有安装实例 ID", "id", install.InstallationInstanceID)
	}

	// 初始化安全存储
	secretsDir := filepath.Join(dataDir, "secrets")
	secStore, err := securestore.NewStore(secretsDir)
	if err != nil {
		logger.Error("初始化安全存储失败", "error", err)
		os.Exit(1)
	}
	defer secStore.Destroy()

	// 初始化活动会话管理器
	sessionMgr := activesession.NewManager(install.ClientID, logger)

	// 初始化会话代理
	sessionProxy := sessionproxy.NewProxy(install.NormalizedServerURL, logger)

	// 初始化服务端绑定管理器
	bindingMgr := serverbinding.NewManager(logger)

	// 初始化 HMAC 签名器（如果有 device_token）
	var deviceSigner *hmacsigner.Signer
	if install.ClientID != "" && secStore.HasSecret("device_token") {
		deviceToken, err := secStore.LoadSecret("device_token", securestore.AADContext{
			ClientID:     install.ClientID,
			SecretType:   "device_token",
			TokenVersion: 1,
		})
		if err != nil {
			logger.Warn("加载 device_token 失败", "error", err)
		} else {
			signingKey := securestore.DeriveHMACKey(deviceToken, install.ClientID)
			deviceSigner = hmacsigner.NewSigner(install.ClientID, signingKey, 1)
		}
	}

	// 初始化 Server API 客户器
	serverClient := serverclient.NewClient(install.NormalizedServerURL, deviceSigner, logger)

	// 初始化 FRPC 管理器
	frpcBinaryPath := filepath.Join(dataDir, "frpc", FRPCBinaryName)
	frpcConfigDir := filepath.Join(dataDir, "frpc")
	frpcMgr := frpc.NewManager(frpcBinaryPath, frpcConfigDir, logger)

	// 初始化 FRPC 监督者
	frpcSupervisor := supervisor.NewSupervisor(frpcMgr, filepath.Join(frpcConfigDir, "current", "frpc.toml"), logger)

	// 初始化配置应用器
	configApplier := configapply.NewApplier(frpcMgr, frpcConfigDir, logger)
	configApplier.Start(ctx)
	defer configApplier.Stop()

	// 初始化 WebSocket 客户端
	var wsHandler wsclient.EventHandler = func(event wsclient.Event) {
		logger.Info("收到 WebSocket 事件", "type", event.Type)
		// 事件处理逻辑在后续实现中完善
	}
	wsClient := wsclient.NewClient(install.NormalizedServerURL, deviceSigner, wsHandler, logger)

	// 初始化本地 API 服务器
	apiServer := localapi.NewServer(localapi.Config{
		Addr:           listenAddr,
		SessionManager: sessionMgr,
		SessionProxy:   sessionProxy,
		Logger:         logger,
		FrpcController: frpcMgr,
		ServerConfig:   &serverConfigImpl{proxy: sessionProxy, binding: bindingMgr, db: db},
	})

	// 启动服务
	var wg sync.WaitGroup

	// 启动 FRPC 监督者
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := frpcSupervisor.Start(); err != nil {
			logger.Warn("FRPC 监督者启动失败（可能 FRPC 未安装）", "error", err)
		}
	}()

	// 启动 WebSocket 客户端（如果已绑定）
	if install.ClientID != "" && install.NormalizedServerURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := wsClient.Connect(); err != nil {
				logger.Warn("WebSocket 连接失败", "error", err)
			}
		}()
	}

	// 启动本地 API 服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := apiServer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("本地 API 服务器退出", "error", err)
		}
	}()

	logger.Info("FRP Client Panel 已就绪",
		"addr", listenAddr,
		"client_id", install.ClientID,
		"server_url", install.NormalizedServerURL,
		"binding_state", install.ServerBindingState,
	)

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("收到关闭信号", "signal", sig)
	case <-ctx.Done():
		logger.Info("Context 已取消")
	}

	// 执行关闭流程
	logger.Info("正在关闭...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 停止 WebSocket
	wsClient.Disconnect()

	// 停止 API 服务器
	apiServer.Stop()

	// 停止 FRPC 监督者和进程
	frpcSupervisor.Stop()
	frpcMgr.Stop()

	// 撤销会话
	sessionMgr.RevokeSession()

	// 等待协程结束
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("所有服务已停止")
	case <-shutdownCtx.Done():
		logger.Warn("关闭超时，强制退出")
	}

	logger.Info("FRP Client Panel 已退出")
}

// defaultDataDir 返回默认数据目录
func defaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("ProgramData"), "FRPClientPanel")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "FRPClientPanel")
	default:
		// Linux
		return "/var/lib/frp-client-panel"
	}
}

// acquireFileLock 获取文件锁，确保单实例运行
func acquireFileLock(dataDir string) (func(), error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	lockPath := filepath.Join(dataDir, ".lock")

	// 尝试创建锁文件
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("打开锁文件失败: %w", err)
	}

	// 平台特定文件锁
	if err := platformFileLock(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("获取文件锁失败: %w", err)
	}

	// 写入 PID
	f.Truncate(0)
	f.Seek(0, 0)
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Sync()

	unlock := func() {
		platformFileUnlock(f)
		f.Close()
		os.Remove(lockPath)
	}

	return unlock, nil
}

// serverConfigImpl 实现 localapi.ServerConfig 接口
type serverConfigImpl struct {
	proxy   *sessionproxy.Proxy
	binding *serverbinding.Manager
	db      *storage.LocalDB
}

func (s *serverConfigImpl) GetServerURL() string {
	return s.proxy.GetServerURL()
}

func (s *serverConfigImpl) SetServerURL(url string) error {
	// 规范化 URL
	normalized, err := serverbinding.NormalizeServerURL(url, true) // 允许 HTTP（开发模式）
	if err != nil {
		return err
	}

	// SSRF 防护
	if err := serverbinding.ValidateSSRFProtection(extractHost(normalized), false); err != nil {
		return err
	}

	s.proxy.SetServerURL(normalized)
	return nil
}

func extractHost(rawURL string) string {
	u, err := newURL(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// simpleURL 简单 URL 解析
type simpleURL struct {
	host string
}

func newURL(rawURL string) (*simpleURL, error) {
	host := rawURL
	if len(host) > 8 && host[:8] == "https://" {
		host = host[8:]
	} else if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	}
	if idx := indexOf(host, ':'); idx > 0 {
		host = host[:idx]
	}
	if idx := indexOf(host, '/'); idx > 0 {
		host = host[:idx]
	}
	return &simpleURL{host: host}, nil
}

func (s *simpleURL) Hostname() string {
	return s.host
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
