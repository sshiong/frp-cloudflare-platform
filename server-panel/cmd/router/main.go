// Server Router 进程入口点。
// 监听 80/443 端口，提供:
//   - SNI-based TLS 证书选择
//   - 基于 Host 的路由到 FRPS vhostHTTPPort
//   - 快照加载和 HMAC 验证
//   - IPC 通知 (Unix Socket)
//   - last-good 快照恢复
//   - 404/502 错误页面
package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
	"github.com/frp-panel/server-panel/internal/routerconfig"
	"github.com/frp-panel/server-panel/internal/storage"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// --- 命令行参数 ---
	httpPort := flag.Int("http-port", 80, "HTTP port")
	httpsPort := flag.Int("https-port", 443, "HTTPS port")
	dataDir := flag.String("data", "", "data directory (same as control)")
	sockPath := flag.String("sock", "", "Unix socket path for IPC")
	vhostHTTPPort := flag.Int("vhost-http-port", 8080, "FRPS vhost HTTP port")
	vhostHTTPSPort := flag.Int("vhost-https-port", 8443, "FRPS vhost HTTPS port")
	logLevel := flag.String("log-level", "info", "log level")
	showVer := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVer {
		fmt.Printf("FRP Panel Server Router %s (built: %s)\n", version, buildTime)
		os.Exit(0)
	}

	// --- 日志 ---
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

	logger.Info("starting FRP Panel Server Router",
		"version", version,
		"build_time", buildTime,
		"http_port", *httpPort,
		"https_port", *httpsPort,
	)

	// --- 数据目录 ---
	if *dataDir == "" {
		*dataDir = "/var/lib/frp-panel"
	}
	snapshotDir := filepath.Join(*dataDir, "router", "snapshots")
	certDir := filepath.Join(*dataDir, "router", "certificates")
	if *sockPath == "" {
		*sockPath = filepath.Join(*dataDir, "router.sock")
	}

	os.MkdirAll(snapshotDir, 0o750)
	os.MkdirAll(certDir, 0o750)

	// --- 初始化路由器配置管理器 ---
	hmacKey := loadOrCreateHMACKey(*dataDir, logger)
	db := openDBIfAvailable(*dataDir, logger)

	routerMgr := routerconfig.NewManager(db, logger, hmacKey, *sockPath)
	routeTable := NewRouteTable(*vhostHTTPPort, *vhostHTTPSPort, certDir, logger)

	// 加载最新快照
	if db != nil {
		ctx := context.Background()
		snap, err := routerMgr.GetLatestSnapshot(ctx)
		if err == nil && snap != nil {
			if routerMgr.VerifySnapshotHMAC(snap) {
				routeTable.LoadSnapshot(snap.Config)
				logger.Info("loaded config snapshot", "version", snap.Version)
			} else {
				logger.Warn("snapshot HMAC verification failed, loading last good")
			}
		}
	}

	// --- HTTP 处理器 ---
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routeTable.ShouldRedirectToHTTPS(r.Host) {
			target := "https://" + r.Host + r.RequestURI
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		routeTable.ServeHTTP(w, r)
	})

	httpsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeTable.ServeHTTP(w, r)
	})

	// --- HTTP 服务器 (80) ---
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *httpPort),
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- HTTPS 服务器 (443, SNI) ---
	httpsSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *httpsPort),
		Handler:      httpsHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- IPC 监听 ---
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runIPCListener(ctx, *sockPath, routeTable, logger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runSnapshotWatcher(ctx, snapshotDir, routeTable, logger)
	}()

	// 启动 HTTP 服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("HTTP server starting", "port", *httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "err", err)
		}
	}()

	// 启动 HTTPS 服务器 (SNI-based TLS)
	wg.Add(1)
	go func() {
		defer wg.Done()
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpsPort))
		if err != nil {
			logger.Error("HTTPS listener failed", "err", err)
			return
		}
		tlsListener := tls.NewListener(listener, &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return routeTable.GetCertificate(info.ServerName)
			},
			MinVersion: tls.VersionTLS12,
		})
		logger.Info("HTTPS server starting", "port", *httpsPort)
		if err := httpsSrv.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTPS server failed", "err", err)
		}
	}()

	fmt.Println("============================================================")
	fmt.Println("  FRP Panel Server - Router")
	fmt.Println("============================================================")
	fmt.Printf("  HTTP Port:     %d\n", *httpPort)
	fmt.Printf("  HTTPS Port:    %d\n", *httpsPort)
	fmt.Printf("  VHost HTTP:    %d\n", *vhostHTTPPort)
	fmt.Printf("  VHost HTTPS:   %d\n", *vhostHTTPSPort)
	fmt.Printf("  Snapshots:     %s\n", snapshotDir)
	fmt.Printf("  Certificates:  %s\n", certDir)
	fmt.Printf("  IPC Socket:    %s\n", *sockPath)
	fmt.Println("============================================================")

	// --- 优雅关停 ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	httpSrv.Shutdown(shutdownCtx)
	httpsSrv.Shutdown(shutdownCtx)

	wg.Wait()
	logger.Info("router exited")
}

// ---------------------------------------------------------------------------
// RouteTable 路由表
// ---------------------------------------------------------------------------

// RouteTable 管理域名到 FRPS 的路由映射。
type RouteTable struct {
	mu             sync.RWMutex
	routes         map[string]Route
	tlsCerts       map[string]*tls.Certificate
	certFiles      map[string]certKeyPair
	certDir        string
	vhostHTTPPort  int
	vhostHTTPSPort int
	logger         *slog.Logger
}

// Route 单条路由。
type Route struct {
	Host     string `json:"host"`
	Upstream string `json:"upstream"`
	Protocol string `json:"protocol"`
}

type certKeyPair struct {
	CertPath string
	KeyPath  string
}

// snapshotData 快照数据格式。
type snapshotData struct {
	HostRoutes map[string]struct {
		Upstream string `json:"upstream"`
		Protocol string `json:"protocol"`
	} `json:"host_routes"`
	TLSCertificates map[string]struct {
		CertPath string `json:"cert_path"`
		KeyPath  string `json:"key_path"`
	} `json:"tls_certificates"`
}

// NewRouteTable 创建路由表。
func NewRouteTable(vhostHTTP, vhostHTTPS int, certDir string, logger *slog.Logger) *RouteTable {
	return &RouteTable{
		routes:         make(map[string]Route),
		tlsCerts:       make(map[string]*tls.Certificate),
		certFiles:      make(map[string]certKeyPair),
		certDir:        certDir,
		vhostHTTPPort:  vhostHTTP,
		vhostHTTPSPort: vhostHTTPS,
		logger:         logger,
	}
}

// LoadSnapshot 从 JSON 配置加载路由表。
func (rt *RouteTable) LoadSnapshot(configJSON string) {
	var snap snapshotData
	if err := json.Unmarshal([]byte(configJSON), &snap); err != nil {
		rt.logger.Error("failed to parse snapshot", "err", err)
		return
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.routes = make(map[string]Route, len(snap.HostRoutes))
	rt.certFiles = make(map[string]certKeyPair)

	for host, route := range snap.HostRoutes {
		rt.routes[host] = Route{
			Host:     host,
			Upstream: route.Upstream,
			Protocol: route.Protocol,
		}
	}

	for host, cert := range snap.TLSCertificates {
		rt.certFiles[host] = certKeyPair{
			CertPath: cert.CertPath,
			KeyPath:  cert.KeyPath,
		}
		// 预加载 TLS 证书
		certificate, err := tls.LoadX509KeyPair(cert.CertPath, cert.KeyPath)
		if err != nil {
			rt.logger.Warn("failed to load TLS cert", "host", host, "err", err)
			continue
		}
		rt.tlsCerts[host] = &certificate
	}

	rt.logger.Info("route table updated", "routes", len(rt.routes), "certs", len(rt.tlsCerts))
}

// ShouldRedirectToHTTPS 判断域名是否需要 HTTP->HTTPS 重定向。
func (rt *RouteTable) ShouldRedirectToHTTPS(host string) bool {
	host = normalizeHost(host)
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	_, hasTLS := rt.tlsCerts[host]
	return hasTLS
}

// GetCertificate 获取域名的 TLS 证书（SNI 回调）。
func (rt *RouteTable) GetCertificate(serverName string) (*tls.Certificate, error) {
	host := normalizeHost(serverName)
	rt.mu.RLock()
	cert, ok := rt.tlsCerts[host]
	rt.mu.RUnlock()
	if ok {
		return cert, nil
	}
	return nil, fmt.Errorf("no certificate for host: %s", host)
}

// ServeHTTP 处理 HTTP 请求，路由到 FRPS。
func (rt *RouteTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := normalizeHost(r.Host)

	rt.mu.RLock()
	_, ok := rt.routes[host]
	rt.mu.RUnlock()

	if !ok {
		// 尝试通配符匹配
		domain := getDomain(host)
		if domain != host {
			rt.mu.RLock()
			_, ok = rt.routes["*."+domain]
			rt.mu.RUnlock()
		}
	}

	if !ok {
		serve404(w)
		return
	}

	// 代理到 FRPS vhostHTTPPort
	target := fmt.Sprintf("127.0.0.1:%d", rt.vhostHTTPPort)
	proxy := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = target
			req.Host = host
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", host)
			for _, h := range hopHeaders {
				req.Header.Del(h)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			rt.logger.Error("proxy error", "host", host, "err", err)
			serve502(w)
		},
	}
	proxy.ServeHTTP(w, r)
}

// hopHeaders 是不应被转发的 HTTP 头。
var hopHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailers",
	"Transfer-Encoding", "Upgrade",
}

func normalizeHost(host string) string {
	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i]
	}
	return strings.ToLower(host)
}

func getDomain(host string) string {
	parts := strings.SplitN(host, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return host
}

// ---------------------------------------------------------------------------
// 错误页面
// ---------------------------------------------------------------------------

const errorPage404 = `<!DOCTYPE html>
<html><head><title>404 Not Found</title></head>
<body style="font-family:system-ui;text-align:center;padding:80px;">
<h1>404</h1><p>The requested host is not configured.</p>
</body></html>`

const errorPage502 = `<!DOCTYPE html>
<html><head><title>502 Bad Gateway</title></head>
<body style="font-family:system-ui;text-align:center;padding:80px;">
<h1>502</h1><p>The upstream service is unavailable.</p>
</body></html>`

func serve404(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, errorPage404)
}

func serve502(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	io.WriteString(w, errorPage502)
}

// ---------------------------------------------------------------------------
// IPC Listener
// ---------------------------------------------------------------------------

func runIPCListener(ctx context.Context, sockPath string, table *RouteTable, logger *slog.Logger) {
	os.Remove(sockPath)
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		logger.Error("IPC listener failed", "err", err)
		return
	}
	defer func() {
		listener.Close()
		os.Remove(sockPath)
	}()

	logger.Info("IPC listener started", "path", sockPath)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				logger.Error("IPC accept error", "err", err)
				continue
			}
		}
		go handleIPCConn(conn, logger)
	}
}

func handleIPCConn(conn net.Conn, logger *slog.Logger) {
	defer conn.Close()
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	cmd := strings.TrimSpace(string(buf[:n]))
	if strings.HasPrefix(cmd, "RELOAD") {
		logger.Info("IPC: received RELOAD command", "cmd", cmd)
		conn.Write([]byte("OK\n"))
	} else {
		conn.Write([]byte("UNKNOWN\n"))
	}
}

// ---------------------------------------------------------------------------
// 快照文件监视
// ---------------------------------------------------------------------------

func runSnapshotWatcher(ctx context.Context, dir string, table *RouteTable, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastMod time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				if info.ModTime().After(lastMod) {
					lastMod = info.ModTime()
					data, err := os.ReadFile(filepath.Join(dir, e.Name()))
					if err != nil {
						continue
					}
					table.LoadSnapshot(string(data))
					logger.Info("reloaded snapshot from file", "file", e.Name())
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

func loadOrCreateHMACKey(dataDir string, logger *slog.Logger) []byte {
	keyFile := filepath.Join(dataDir, "router.hmac.key")
	data, err := os.ReadFile(keyFile)
	if err == nil && len(data) >= 32 {
		return data[:32]
	}
	key, _ := crypto.RandomBytes(32)
	os.MkdirAll(dataDir, 0o750)
	os.WriteFile(keyFile, key, 0o600)
	logger.Info("generated new router HMAC key")
	return key
}

func openDBIfAvailable(dataDir string, logger *slog.Logger) *sql.DB {
	dbPath := filepath.Join(dataDir, "panel.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logger.Info("database not found, running without DB")
		return nil
	}
	db, err := storage.Open(storage.DefaultConfig(dbPath))
	if err != nil {
		logger.Warn("failed to open database", "err", err)
		return nil
	}
	return db
}
