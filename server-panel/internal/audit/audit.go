// Package audit 提供审计日志记录功能。
// 所有重要操作均需记录，敏感字段自动脱敏。
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

// sensitiveFields 是需要脱敏的字段名列表（不区分大小写）。
var sensitiveFields = []string{
	"password", "token", "secret", "key", "api_key", "api_token",
	"private_key", "cert_pem", "key_pem", "chain_pem", "token_enc",
	"authorization", "cookie", "session",
}

// Logger 审计日志记录器。
type Logger struct {
	db     *sql.DB
	logger *slog.Logger
}

// New 创建审计日志记录器。
func New(db *sql.DB, logger *slog.Logger) *Logger {
	return &Logger{db: db, logger: logger}
}

// Entry 审计日志条目。
type Entry struct {
	RequestID  string                 `json:"request_id"`
	UserID     string                 `json:"user_id"`
	Action     string                 `json:"action"`
	TargetType string                 `json:"target_type"`
	TargetID   string                 `json:"target_id"`
	Detail     map[string]interface{} `json:"detail"`
	IP         string                 `json:"ip"`
}

// Log 记录一条审计日志。
func (l *Logger) Log(ctx context.Context, entry Entry) {
	// 脱敏处理
	sanitized := SanitizeDetail(entry.Detail)
	detailJSON, err := json.Marshal(sanitized)
	if err != nil {
		l.logger.Error("failed to marshal audit detail", "err", err)
		detailJSON = []byte("{}")
	}

	_, err = l.db.ExecContext(ctx, `
		INSERT INTO audit_logs (request_id, actor_type, actor_id, action, resource_type, resource_id, result, metadata_json, browser_source_ip)
		VALUES (?, 'user', ?, ?, ?, ?, 'success', ?, ?)
	`, entry.RequestID, entry.UserID, entry.Action, entry.TargetType, entry.TargetID,
		string(detailJSON), entry.IP)

	if err != nil {
		l.logger.Error("failed to write audit log",
			"action", entry.Action,
			"user_id", entry.UserID,
			"err", err)
	}
}

// LogSimple 记录一条简化的审计日志。
func (l *Logger) LogSimple(ctx context.Context, requestID, userID, action, targetType, targetID, ip string) {
	l.Log(ctx, Entry{
		RequestID:  requestID,
		UserID:     userID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     nil,
		IP:         ip,
	})
}

// SanitizeDetail 对详情 map 进行脱敏处理。
func SanitizeDetail(detail map[string]interface{}) map[string]interface{} {
	if detail == nil {
		return nil
	}
	result := make(map[string]interface{}, len(detail))
	for k, v := range detail {
		if isSensitive(k) {
			result[k] = "***REDACTED***"
		} else if subMap, ok := v.(map[string]interface{}); ok {
			result[k] = SanitizeDetail(subMap)
		} else {
			result[k] = v
		}
	}
	return result
}

func isSensitive(field string) bool {
	lower := strings.ToLower(field)
	for _, sf := range sensitiveFields {
		if strings.Contains(lower, sf) {
			return true
		}
	}
	return false
}

// Query 查询审计日志。
type Query struct {
	UserID     string
	Action     string
	TargetType string
	Since      time.Time
	Until      time.Time
	Limit      int
	Offset     int
}

// Result 查询结果。
type Result struct {
	Entries []AuditEntry
	Total   int
}

// AuditEntry 数据库中的审计日志条目。
type AuditEntry struct {
	ID         int64  `json:"id"`
	RequestID  string `json:"request_id"`
	UserID     string `json:"user_id"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	CreatedAt  string `json:"created_at"`
}

// QueryLogs 查询审计日志。
func (l *Logger) QueryLogs(ctx context.Context, q Query) (Result, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 500 {
		q.Limit = 500
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	if q.UserID != "" {
		conditions = append(conditions, "actor_id = ?")
		args = append(args, q.UserID)
	}
	if q.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, q.Action)
	}
	if q.TargetType != "" {
		conditions = append(conditions, "resource_type = ?")
		args = append(args, q.TargetType)
	}
	if !q.Since.IsZero() {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, q.Since.Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, q.Until.Format(time.RFC3339))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs " + where
	if err := l.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return Result{}, err
	}

	// 查询数据
	dataQuery := "SELECT id, COALESCE(request_id,''), actor_id, action, COALESCE(resource_type,''), COALESCE(resource_id,''), COALESCE(metadata_json,'{}'), COALESCE(browser_source_ip,''), created_at FROM audit_logs " +
		where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	dataArgs := append(args, q.Limit, q.Offset)

	rows, err := l.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var detailStr string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.UserID, &e.Action, &e.TargetType, &e.TargetID, &detailStr, &e.IP, &e.CreatedAt); err != nil {
			return Result{}, err
		}
		e.Detail = detailStr
		entries = append(entries, e)
	}

	return Result{Entries: entries, Total: total}, nil
}
