package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/frp-panel/server-panel/internal/routerconfig"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Command line flags
	var (
		httpPort     = flag.Int("http-port", 80, "HTTP port")
		httpsPort    = flag.Int("https-port", 443, "HTTPS port")
		snapshotDir  = flag.String("snapshot-dir", "", "Snapshot directory")
		certDir      = flag.String("cert-dir", "", "Certificate directory")
		vhostHTTPPort = flag.Int("vhost-http-port", 8080, "FRPS vhost HTTP port")
		logLevel     = flag.String("log-level", "info", "Log level")
		showVer      = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	// Show version
	if *showVer {
		fmt.Printf("FRP Panel Server Router %s (built: %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Setup logger
	var logLevelVar slog.Level
	switch *logLevel {
	case "debug":
		logLevelVar = slog.LevelDebug
	case "info":
		logLevelVar = slog.LevelInfo
	case "warn":
		logLevelVar = slog.LevelWarn
	case "error":
		logLevelVar = slog.LevelError
	default:
		logLevelVar = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevelVar,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting FRP Panel Server Router",
		"version", version,
		"build_time", buildTime,
		"http_port", *httpPort,
		"https_port", *httpsPort,
	)

	// Default paths
	if *snapshotDir == "" {
		*snapshotDir = "/var/lib/frp-panel/router/snapshots"
	}
	if *certDir == "" {
		*certDir = "/var/lib/frp-panel/router/certificates"
	}

	// Ensure directories exist
	if err := os.MkdirAll(*snapshotDir, 0750); err != nil {
		logger.Error("Failed to create snapshot directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*certDir, 0750); err != nil {
		logger.Error("Failed to create certificate directory", "error", err)
		os.Exit(1)
	}

	// Initialize router config manager
	routerMgr := routerconfig.NewManager(*snapshotDir, *certDir, logger)

	// Load last good snapshot
	if err := routerMgr.LoadLastGood(); err != nil {
		logger.Warn("No last good snapshot found, starting with empty config", "error", err)
	}

	// Create route handler
	handler := newRouterHandler(routerMgr, *vhostHTTPPort, logger)

	// HTTP server (redirect to HTTPS or serve directly)
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *httpPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if domain has HTTP redirect enabled
			if routerMgr.ShouldRedirect(r.Host) {
				target := "https://" + r.Host + r.RequestURI
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
			// Otherwise proxy to FRPS
			handler.ServeHTTP(w, r)
		}),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// HTTPS server with TLS
	httpsSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *httpsPort),
		Handler: handler,
		TLSConfig: &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return routerMgr.GetCertificate(info.ServerName)
			},
			MinVersion: tls.VersionTLS12,
		},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start snapshot watcher
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		routerMgr.WatchSnapshots(ctx)
	}()

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("HTTP server starting", "port", *httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
		}
	}()

	// Start HTTPS server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("HTTPS server starting", "port", *httpsPort)
		if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTPS server failed", "error", err)
		}
	}()

	// Print startup info
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              FRP Panel Server - Router                       ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Version:       %-44s ║\n", version)
	fmt.Printf("║  HTTP Port:     %-44d ║\n", *httpPort)
	fmt.Printf("║  HTTPS Port:    %-44d ║\n", *httpsPort)
	fmt.Printf("║  VHost HTTP:    %-44d ║\n", *vhostHTTPPort)
	fmt.Printf("║  Snapshots:     %-44s ║\n", *snapshotDir)
	fmt.Printf("║  Certificates:  %-44s ║\n", *certDir)
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down router...")

	// Cancel snapshot watcher
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced to shutdown", "error", err)
	}

	if err := httpsSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTPS server forced to shutdown", "error", err)
	}

	wg.Wait()
	logger.Info("Router exited")
}

// routerHandler handles incoming HTTP requests
type routerHandler struct {
	manager      *routerconfig.Manager
	vhostPort    int
	logger       *slog.Logger
}

func newRouterHandler(manager *routerconfig.Manager, vhostPort int, logger *slog.Logger) *routerHandler {
	return &routerHandler{
		manager:   manager,
		vhostPort: vhostPort,
		logger:    logger,
	}
}

func (h *routerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	// Get route for host
	route := h.manager.GetRoute(host)
	if route == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Proxy to FRPS vhost HTTP port
	target := fmt.Sprintf("127.0.0.1:%d", h.vhostPort)
	proxy := &httpProxy{
		target:  target,
		host:    host,
		route:   route,
		logger:  h.logger,
	}
	proxy.ServeHTTP(w, r)
}

// httpProxy proxies requests to FRPS
type httpProxy struct {
	target string
	host   string
	route  *routerconfig.Route
	logger *slog.Logger
}

func (p *httpProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = p.target
			req.Host = p.host

			// Set forwarded headers
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", p.host)

			// Remove hop-by-hop headers
			for _, h := range hopHeaders {
				req.Header.Del(h)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			p.logger.Error("Proxy error", "error", err, "host", p.host)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// hopHeaders are headers that should not be forwarded
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}
