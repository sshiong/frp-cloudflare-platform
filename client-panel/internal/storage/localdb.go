package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// LocalDB 封装客户端本地 SQLite 数据库
// 仅存储非敏感数据：安装实例、配置状态、运行状态、绑定别名
// 禁止存储：密码、Session、Token、Cookie
type LocalDB struct {
	db     *sql.DB
	logger *slog.Logger
}

// InstallationRecord 安装实例记录
type InstallationRecord struct {
	InstallationInstanceID string
	ServerInstanceID       string
	NormalizedServerURL    string
	ServerBindingState     string // unbound, binding, bound, switching_server, credential_revoked, unbinding
	OwnerUserID            string
	ClientID               string
	ServerBindingRevision  int64
	ConfigSigningPublicKey []byte
	ConfigSigningKeyID     string
	TLSTrustMode           string // system, pinned_spki, custom_ca
	PinnedSPKISHA256       string
	CustomCAPath           string
	DesiredConfigVersion   int64
	AppliedConfigVersion   int64
	LastFailedConfigVer    int64
	LastSyncAt             string
	LastHeartbeatAt        string
	ClientPanelVersion     string
	FRPCVersion            string
	ProtocolVersion        int
	ConfigSchemaVersion    int
}

// ConfigStateRecord 配置状态记录
type ConfigStateRecord struct {
	ConfigVersion  int64
	ConfigHash     string
	ConfigBody     string
	SignatureValid bool
	ApplyStatus    string // pending, applied, failed, rolled_back
	AppliedAt      string
	ErrorSummary   string
}

// FRPCRuntimeState FRPC 运行时状态
type FRPCRuntimeState struct {
	PID               int
	StartTimeUnix     int64
	BinaryPath        string
	BinaryHash        string
	BinaryVersion     string
	AdminAddr         string
	AdminUser         string
	AdminPassEncrypted []byte
	Status            string // stopped, starting, running, stopping, error
	LastError         string
	UptimeSeconds     int64
	ProxyCount        int
}

// Open 打开本地 SQLite 数据库
// 设置 WAL 模式、外键约束和忙碌超时
func Open(ctx context.Context, dbPath string, logger *slog.Logger) (*LocalDB, error) {
	// 确保数据库目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// DSN 参数：
	// _journal_mode=WAL     - 预写日志模式，提高并发读性能
	// _foreign_keys=ON      - 启用外键约束
	// _busy_timeout=5000    - 写锁等待超时 5 秒
	// _synchronous=FULL     - 完全同步，保证数据安全
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000&_synchronous=FULL", dbPath)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 验证连接
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库连接验证失败: %w", err)
	}

	// 设置连接池参数（SQLite 单写者，适当限制连接数）
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // SQLite 连接不过期

	ldb := &LocalDB{db: db, logger: logger}

	// 执行迁移
	if err := RunMigrations(ctx, db, logger); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	logger.Info("本地数据库已打开", "path", dbPath)
	return ldb, nil
}

// Close 关闭数据库
func (ldb *LocalDB) Close() error {
	return ldb.db.Close()
}

// DB 返回底层 sql.DB，供需要直接访问的场景使用
func (ldb *LocalDB) DB() *sql.DB {
	return ldb.db
}

// GetInstallation 获取安装实例记录
func (ldb *LocalDB) GetInstallation(ctx context.Context) (*InstallationRecord, error) {
	rec := &InstallationRecord{}
	err := ldb.db.QueryRowContext(ctx, `SELECT
		installation_instance_id, server_instance_id, normalized_server_url,
		server_binding_state, owner_user_id, client_id, server_binding_revision,
		config_signing_public_key, config_signing_key_id,
		tls_trust_mode, pinned_spki_sha256, custom_ca_path,
		desired_config_version, applied_config_version, last_failed_config_version,
		last_sync_at, last_heartbeat_at,
		client_panel_version, frpc_version, protocol_version, config_schema_version
	FROM client_installation WHERE id = 1`).Scan(
		&rec.InstallationInstanceID, &rec.ServerInstanceID, &rec.NormalizedServerURL,
		&rec.ServerBindingState, &rec.OwnerUserID, &rec.ClientID, &rec.ServerBindingRevision,
		&rec.ConfigSigningPublicKey, &rec.ConfigSigningKeyID,
		&rec.TLSTrustMode, &rec.PinnedSPKISHA256, &rec.CustomCAPath,
		&rec.DesiredConfigVersion, &rec.AppliedConfigVersion, &rec.LastFailedConfigVer,
		&rec.LastSyncAt, &rec.LastHeartbeatAt,
		&rec.ClientPanelVersion, &rec.FRPCVersion, &rec.ProtocolVersion, &rec.ConfigSchemaVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("读取安装实例失败: %w", err)
	}
	return rec, nil
}

// UpdateInstallation 更新安装实例的指定字段
func (ldb *LocalDB) UpdateInstallation(ctx context.Context, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	query := "UPDATE client_installation SET updated_at = ?"
	args := []interface{}{time.Now().UTC().Format(time.RFC3339)}

	for col, val := range updates {
		query += fmt.Sprintf(", %s = ?", col)
		args = append(args, val)
	}
	query += " WHERE id = 1"

	_, err := ldb.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("更新安装实例失败: %w", err)
	}
	return nil
}

// SetInstallationID 设置安装实例 ID（仅首次）
func (ldb *LocalDB) SetInstallationID(ctx context.Context, id string) error {
	_, err := ldb.db.ExecContext(ctx,
		"UPDATE client_installation SET installation_instance_id = ?, updated_at = ? WHERE id = 1 AND installation_instance_id = ''",
		id, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("设置安装实例 ID 失败: %w", err)
	}
	return nil
}

// SaveConfigState 保存配置状态快照
func (ldb *LocalDB) SaveConfigState(ctx context.Context, rec *ConfigStateRecord) error {
	_, err := ldb.db.ExecContext(ctx,
		`INSERT INTO client_config_state (config_version, config_hash, config_body, signature_valid, apply_status, applied_at, error_summary)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ConfigVersion, rec.ConfigHash, rec.ConfigBody,
		boolToInt(rec.SignatureValid), rec.ApplyStatus, rec.AppliedAt, rec.ErrorSummary)
	if err != nil {
		return fmt.Errorf("保存配置状态失败: %w", err)
	}
	return nil
}

// UpdateConfigApplyStatus 更新配置应用状态
func (ldb *LocalDB) UpdateConfigApplyStatus(ctx context.Context, version int64, status string, errorSummary string) error {
	_, err := ldb.db.ExecContext(ctx,
		"UPDATE client_config_state SET apply_status = ?, error_summary = ? WHERE config_version = ?",
		status, errorSummary, version)
	if err != nil {
		return fmt.Errorf("更新配置应用状态失败: %w", err)
	}
	return nil
}

// GetLatestConfigState 获取最新已应用的配置状态
func (ldb *LocalDB) GetLatestConfigState(ctx context.Context) (*ConfigStateRecord, error) {
	rec := &ConfigStateRecord{}
	var sigValid int
	err := ldb.db.QueryRowContext(ctx,
		`SELECT config_version, config_hash, config_body, signature_valid, apply_status, applied_at, error_summary
		 FROM client_config_state WHERE apply_status = 'applied' ORDER BY config_version DESC LIMIT 1`).
		Scan(&rec.ConfigVersion, &rec.ConfigHash, &rec.ConfigBody,
			&sigValid, &rec.ApplyStatus, &rec.AppliedAt, &rec.ErrorSummary)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("获取最新配置状态失败: %w", err)
	}
	rec.SignatureValid = sigValid != 0
	return rec, nil
}

// UpdateFRPCRuntimeState 更新 FRPC 运行时状态
func (ldb *LocalDB) UpdateFRPCRuntimeState(ctx context.Context, state *FRPCRuntimeState) error {
	_, err := ldb.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO frpc_runtime_state (id, pid, start_time_unix, binary_path, binary_hash, binary_version,
		 admin_addr, admin_user, admin_pass_encrypted, status, last_error, uptime_seconds, proxy_count, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		state.PID, state.StartTimeUnix, state.BinaryPath, state.BinaryHash, state.BinaryVersion,
		state.AdminAddr, state.AdminUser, state.AdminPassEncrypted,
		state.Status, state.LastError, state.UptimeSeconds, state.ProxyCount,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("更新 FRPC 运行时状态失败: %w", err)
	}
	return nil
}

// GetFRPCRuntimeState 获取 FRPC 运行时状态
func (ldb *LocalDB) GetFRPCRuntimeState(ctx context.Context) (*FRPCRuntimeState, error) {
	state := &FRPCRuntimeState{}
	err := ldb.db.QueryRowContext(ctx,
		`SELECT pid, start_time_unix, binary_path, binary_hash, binary_version,
		 admin_addr, admin_user, admin_pass_encrypted, status, last_error, uptime_seconds, proxy_count
		 FROM frpc_runtime_state WHERE id = 1`).
		Scan(&state.PID, &state.StartTimeUnix, &state.BinaryPath, &state.BinaryHash, &state.BinaryVersion,
			&state.AdminAddr, &state.AdminUser, &state.AdminPassEncrypted,
			&state.Status, &state.LastError, &state.UptimeSeconds, &state.ProxyCount)
	if err != nil {
		return nil, fmt.Errorf("获取 FRPC 运行时状态失败: %w", err)
	}
	return state, nil
}

// AddServerAlias 添加服务端地址别名
func (ldb *LocalDB) AddServerAlias(ctx context.Context, aliasURL string) error {
	_, err := ldb.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO server_binding_aliases (alias_url, verified_at) VALUES (?, ?)",
		aliasURL, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("添加服务端别名失败: %w", err)
	}
	return nil
}

// GetServerAliases 获取所有已验证的服务端地址别名
func (ldb *LocalDB) GetServerAliases(ctx context.Context) ([]string, error) {
	rows, err := ldb.db.QueryContext(ctx, "SELECT alias_url FROM server_binding_aliases ORDER BY created_at")
	if err != nil {
		return nil, fmt.Errorf("查询服务端别名失败: %w", err)
	}
	defer rows.Close()

	var aliases []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("扫描服务端别名失败: %w", err)
		}
		aliases = append(aliases, url)
	}
	return aliases, rows.Err()
}

// boolToInt 将 bool 转为 SQLite 整数
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
