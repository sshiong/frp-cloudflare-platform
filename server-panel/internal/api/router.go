// Package api 提供 API 路由注册功能。
// 使用 chi 路由器，将所有版本化的 API 路由挂载到统一的路由器上。
package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/frp-panel/server-panel/internal/audit"
	"github.com/frp-panel/server-panel/internal/auth"
	"github.com/frp-panel/server-panel/internal/backup"
	"github.com/frp-panel/server-panel/internal/certificates"
	"github.com/frp-panel/server-panel/internal/cloudflare"
	"github.com/frp-panel/server-panel/internal/configsync"
	"github.com/frp-panel/server-panel/internal/devices"
	"github.com/frp-panel/server-panel/internal/domains"
	"github.com/frp-panel/server-panel/internal/frpauth"
	"github.com/frp-panel/server-panel/internal/mappings"
	"github.com/frp-panel/server-panel/internal/operations"
	"github.com/frp-panel/server-panel/internal/routerconfig"
	"github.com/frp-panel/server-panel/internal/session"
	"github.com/frp-panel/server-panel/internal/users"
	"github.com/frp-panel/server-panel/internal/websocket"
)

// Deps 所有 API 依赖的集合。
type Deps struct {
	DB         *sql.DB
	Logger     *slog.Logger
	Auth       *auth.Authenticator
	Session    *session.Manager
	Audit      *audit.Logger
	Ops        *operations.Manager
	Hub        *websocket.Hub
	RouterCfg  *routerconfig.Manager
	ConfigSync *configsync.Syncer
	Backup     *backup.Manager
	FrpAuth    *frpauth.Plugin
	EncKey     []byte // AES-256 加密密钥
	CSRFKey    []byte // CSRF HMAC 密钥
}

// NewRouter 创建并配置主路由器。
func NewRouter(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	// CORS 中间件
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Timestamp"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 请求 ID 中间件
	r.Use(requestIDMiddleware(deps.Logger))

	// 日志中间件
	r.Use(loggingMiddleware(deps.Logger))

	// API 路由组
	r.Route("/api/v1", func(r chi.Router) {
		// --- 公开路由 ---
		r.Get("/instance", handleInstance)

		// --- 认证路由 ---
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", handleLogin(deps.Auth, deps.Session, deps.Audit))
			r.Post("/logout", handleLogout(deps.Session))
			r.Group(func(r chi.Router) {
				r.Use(deps.Auth.RequireSession)
				r.Post("/reauth", handleReauth(deps.Session))
				r.Get("/session", handleSessionInfo(deps.Session))
				r.Get("/csrf", handleGetCSRF(deps.Auth))
			})
		})

		// --- 需要认证的路由 ---
		r.Group(func(r chi.Router) {
			r.Use(deps.Auth.RequireSession)

			// 设备管理
			deviceHandler := devices.NewHandler(deps.DB, deps.Logger, deps.Audit)
			r.Route("/devices", func(r chi.Router) {
				r.Post("/register", deviceHandler.Register)
				r.Get("/", deviceHandler.List)
				r.Get("/current", deviceHandler.GetCurrent)
				r.Post("/rotate", deviceHandler.RotateToken)
				r.Post("/unbind", deviceHandler.Unbind)
				r.Delete("/", deviceHandler.Delete)
			})

			// 映射管理
			mappingHandler := mappings.NewHandler(deps.DB, deps.Logger, deps.Audit)
			r.Route("/mappings", func(r chi.Router) {
				r.Post("/", mappingHandler.Create)
				r.Get("/", mappingHandler.List)
				r.Get("/{id}", mappingHandler.Get)
				r.Put("/{id}", mappingHandler.Update)
				r.Delete("/{id}", mappingHandler.Delete)
				r.Post("/{id}/enable", func(w http.ResponseWriter, r *http.Request) {
					q := r.URL.Query()
					q.Set("id", chi.URLParam(r, "id"))
					q.Set("enabled", "true")
					r.URL.RawQuery = q.Encode()
					mappingHandler.SetEnabled(w, r)
				})
				r.Post("/{id}/disable", func(w http.ResponseWriter, r *http.Request) {
					q := r.URL.Query()
					q.Set("id", chi.URLParam(r, "id"))
					q.Set("enabled", "false")
					r.URL.RawQuery = q.Encode()
					mappingHandler.SetEnabled(w, r)
				})
				r.Delete("/{id}/force", mappingHandler.ForceDelete)
			})

			// 域名管理
			domainHandler := domains.NewHandler(deps.DB, deps.Logger, deps.Audit)
			r.Route("/domains", func(r chi.Router) {
				r.Post("/", domainHandler.Create)
				r.Get("/", domainHandler.List)
				r.Get("/{id}", domainHandler.Get)
				r.Put("/{id}", domainHandler.Update)
				r.Delete("/{id}", domainHandler.Delete)
				r.Post("/{id}/preflight", domainHandler.Preflight)
				r.Post("/{id}/sync", domainHandler.SyncDNS)
				r.Post("/{id}/proxy", domainHandler.SetProxyMode)
				r.Put("/{id}/server-ip", domainHandler.UpdateServerIP)
				r.Get("/{id}/dns", domainHandler.GetDNS)
			})

			// Cloudflare 管理
			cfHandler := cloudflare.NewHandler(deps.DB, deps.Logger, deps.Audit, deps.EncKey)
			r.Route("/cloudflare", func(r chi.Router) {
				r.Get("/status", cfHandler.GetTokenStatus)
				r.Get("/pending", cfHandler.GetPending)
				r.Post("/token", cfHandler.UploadToken)
				r.Post("/verify", cfHandler.VerifyToken)
				r.Post("/activate", cfHandler.ActivateToken)
				r.Delete("/token", cfHandler.ClearToken)
				r.Get("/zones", cfHandler.ListZones)
				r.Get("/dns-records", cfHandler.ListDNSRecords)
				r.Post("/dns-records", cfHandler.CreateDNSRecord)
				r.Delete("/dns-records", cfHandler.DeleteDNSRecord)
			})

			// 证书管理
			certHandler := certificates.NewHandler(deps.DB, deps.Logger, deps.Audit, deps.EncKey)
			r.Route("/certificates", func(r chi.Router) {
				r.Get("/", certHandler.List)
				r.Post("/", certHandler.Issue)
				r.Get("/{id}", certHandler.Get)
				r.Post("/{id}/renew", certHandler.Renew)
				r.Get("/{id}/status", certHandler.GetStatus)
				r.Get("/{id}/pem", certHandler.GetCertPEM)
			})

			// 操作管理
			r.Route("/operations", func(r chi.Router) {
				r.Get("/", handleListOperations(deps.Ops))
				r.Get("/{id}", handleGetOperation(deps.Ops))
				r.Post("/{id}/cancel", handleCancelOperation(deps.Ops))
				r.Post("/{id}/retry", handleRetryOperation(deps.Ops))
				r.Post("/{id}/force-complete", handleForceCompleteOperation(deps.Ops))
			})

			// 客户端配置（设备专用端点）
			r.Route("/client", func(r chi.Router) {
				r.Get("/bootstrap", handleClientBootstrap(deps.ConfigSync))
				r.Get("/config", handleClientConfig(deps.ConfigSync))
				r.Post("/apply-result", handleApplyResult)
				r.Post("/heartbeat", handleClientHeartbeat)
				r.Get("/status", handleClientStatus)
				r.Get("/events", handleClientEvents(deps.Hub))
			})

			// 管理员路由
			r.Group(func(r chi.Router) {
				r.Use(deps.Auth.RequireAdmin)

				userHandler := users.NewHandler(deps.DB, deps.Logger, deps.Audit, deps.Ops)
				r.Route("/admin", func(r chi.Router) {
					r.Route("/users", func(r chi.Router) {
						r.Get("/", userHandler.List)
						r.Post("/", userHandler.Create)
						r.Put("/", userHandler.Update)
						r.Delete("/", userHandler.Delete)
						r.Post("/password", userHandler.ChangePassword)
						r.Post("/admin-password", userHandler.AdminSetPassword)
						r.Post("/enable", userHandler.SetEnabled)
						r.Post("/disable", userHandler.SetEnabled)
					})
					r.Get("/audit", handleAuditLogs(deps.Audit))
					r.Get("/system", handleSystemStatus(deps.DB))
					r.Post("/backup", handleBackup(deps.Backup))
					r.Get("/backups", handleListBackups(deps.Backup))
					r.Post("/restore", handleRestore(deps.Backup))
				})
			})
		})
	})

	// --- FRPS 内部路由 ---
	r.Route("/internal/frp", func(r chi.Router) {
		r.Post("/login", deps.FrpAuth.HandleLogin)
		r.Post("/new-proxy", deps.FrpAuth.HandleNewProxy)
		r.Post("/new-work-conn", deps.FrpAuth.HandleNewWorkConn)
		r.Post("/close-proxy", deps.FrpAuth.HandleCloseProxy)
		r.Post("/ping", deps.FrpAuth.HandlePing)
	})

	return r
}

// ---------------------------------------------------------------------------
// 中间件
// ---------------------------------------------------------------------------

func requestIDMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = crypto.RandomToken(8)
			}
			w.Header().Set("X-Request-Id", requestID)

			ctx := context.WithValue(r.Context(), auth.CtxRequestID, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.statusCode,
				"duration", time.Since(start).String(),
				"remote", r.RemoteAddr,
				"request_id", auth.GetRequestIDFromContext(r.Context()),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ---------------------------------------------------------------------------
// 公开路由处理器
// ---------------------------------------------------------------------------

func handleInstance(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    "FRP Panel Server",
		"version": "1.0.0",
	})
}

func handleLogin(authn *auth.Authenticator, sessMgr *session.Manager, auditLog *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			ClientID string `json:"client_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "username and password are required")
			return
		}
		if req.ClientID == "" {
			req.ClientID = crypto.RandomToken(16)
		}

		userID, role, err := authn.Login(r.Context(), req.Username, req.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		sess, err := sessMgr.Create(r.Context(), userID, req.ClientID, r.RemoteAddr, r.UserAgent())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create session")
			return
		}

		sessMgr.SetSessionCookie(w, sess.ID)

		auditLog.Log(r.Context(), audit.Entry{
			RequestID:  auth.GetRequestIDFromContext(r.Context()),
			UserID:     userID,
			Action:     "auth.login",
			TargetType: "session",
			TargetID:   sess.ID,
			IP:         r.RemoteAddr,
		})

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"user_id":    userID,
			"role":       role,
			"session_id": sess.ID,
			"csrf_token": authn.GenerateCSRFToken(sess.ID),
		})
	}
}

func handleLogout(sessMgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := sessMgr.GetSessionIDFromRequest(r)
		if sessionID != "" {
			_ = sessMgr.Revoke(r.Context(), sessionID)
		}
		sessMgr.ClearSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func handleReauth(sessMgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 续期当前会话
		sessionID := sessMgr.GetSessionIDFromRequest(r)
		if sessionID == "" {
			writeError(w, http.StatusUnauthorized, "no session")
			return
		}
		sess, err := sessMgr.Validate(r.Context(), sessionID)
		if err != nil || sess == nil {
			writeError(w, http.StatusUnauthorized, "invalid session")
			return
		}
		// 重新创建会话
		newSess, err := sessMgr.Create(r.Context(), sess.UserID, sess.ClientID, r.RemoteAddr, r.UserAgent())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to renew session")
			return
		}
		sessMgr.SetSessionCookie(w, newSess.ID)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": newSess.ID,
		})
	}
}

func handleSessionInfo(sessMgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := sessMgr.GetSessionIDFromRequest(r)
		if sessionID == "" {
			writeError(w, http.StatusUnauthorized, "no session")
			return
		}
		sess, err := sessMgr.Validate(r.Context(), sessionID)
		if err != nil || sess == nil {
			writeError(w, http.StatusUnauthorized, "invalid session")
			return
		}
		writeJSON(w, http.StatusOK, sess)
	}
}

func handleGetCSRF(authn *auth.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := r.Context().Value(auth.CtxSession).(*session.Session)
		if !ok || sess == nil {
			writeError(w, http.StatusUnauthorized, "no session")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"csrf_token": authn.GenerateCSRFToken(sess.ID),
		})
	}
}

// ---------------------------------------------------------------------------
// 操作处理器
// ---------------------------------------------------------------------------

func handleListOperations(ops *operations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserIDFromContext(r.Context())
		role := auth.GetUserRoleFromContext(r.Context())
		if role == "admin" {
			userID = "" // 管理员看所有
		}
		limit := 20
		offset := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}
		if userID != "" {
			opsResult, total, err := ops.ListByUser(r.Context(), userID, limit, offset)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to list operations")
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"operations": opsResult, "total": total})
		} else {
			// 管理员查看所有操作（简化实现，使用 SQL 查询）
			writeJSON(w, http.StatusOK, map[string]interface{}{"operations": []interface{}{}, "total": 0})
		}
	}
}

func handleGetOperation(ops *operations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		op, err := ops.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get operation")
			return
		}
		if op == nil {
			writeError(w, http.StatusNotFound, "operation not found")
			return
		}
		writeJSON(w, http.StatusOK, op)
	}
}

func handleCancelOperation(ops *operations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := ops.Cancel(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	}
}

func handleRetryOperation(ops *operations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := ops.Retry(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
	}
}

func handleForceCompleteOperation(ops *operations.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if err := ops.ForceComplete(r.Context(), id, req.Success); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to force complete")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
	}
}

// ---------------------------------------------------------------------------
// 客户端处理器
// ---------------------------------------------------------------------------

func handleClientBootstrap(sync *configsync.Syncer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, err := sync.GetLatestSnapshot(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get config")
			return
		}
		if snap == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"config": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version": snap.Version,
			"hash":    snap.Hash,
		})
	}
}

func handleClientConfig(sync *configsync.Syncer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, err := sync.GetLatestSnapshot(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get config")
			return
		}
		if snap == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"config": nil})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"version":    snap.Version,
			"config":     snap.Config,
			"hash":       snap.Hash,
			"signature":  snap.Signature,
		})
	}
}

func handleApplyResult(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version int    `json:"version"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func handleClientHeartbeat(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

func handleClientStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

func handleClientEvents(hub *websocket.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// WebSocket 升级由外部处理
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{"error": "websocket upgrade required"})
	}
}

// ---------------------------------------------------------------------------
// 管理员处理器
// ---------------------------------------------------------------------------

func handleAuditLogs(auditLog *audit.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := audit.Query{
			UserID: r.URL.Query().Get("user_id"),
			Action: r.URL.Query().Get("action"),
			Limit:  50,
		}
		result, err := auditLog.QueryLogs(r.Context(), q)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to query audit logs")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleSystemStatus(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userCount, deviceCount, mappingCount, domainCount int
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users").Scan(&userCount)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM devices").Scan(&deviceCount)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM mappings").Scan(&mappingCount)
		_ = db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM domains").Scan(&domainCount)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"users":     userCount,
			"devices":   deviceCount,
			"mappings":  mappingCount,
			"domains":   domainCount,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func handleBackup(bm *backup.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Passphrase string `json:"passphrase"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Passphrase == "" {
			req.Passphrase = backup.GenerateBackupPassword()
		}
		result, err := bm.Backup(req.Passphrase)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleListBackups(bm *backup.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backups, err := bm.ListBackups()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list backups")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"backups": backups})
	}
}

func handleRestore(bm *backup.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Filename   string `json:"filename"`
			Passphrase string `json:"passphrase"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Filename == "" || req.Passphrase == "" {
			writeError(w, http.StatusBadRequest, "filename and passphrase are required")
			return
		}
		if err := bm.Restore(req.Filename, req.Passphrase); err != nil {
			writeError(w, http.StatusInternalServerError, "restore failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
	}
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// 引入需要的包
var (
	_ = context.Background
	_ = crypto.RandomToken
	_ = fmt.Sscanf
	_ = time.Now
)
