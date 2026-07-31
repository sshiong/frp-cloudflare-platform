package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// Migration 代表一个数据库迁移步骤
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// clientMigrations 是客户端本地数据库的全部迁移定义
// 版本号必须严格单调递增，禁止回退或重用
var clientMigrations = []Migration{
	{
		Version: 1,
		Name:    "create_client_installation",
		SQL: `CREATE TABLE IF NOT EXISTS client_installation (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			installation_instance_id TEXT NOT NULL UNIQUE,
			server_instance_id TEXT NOT NULL DEFAULT '',
			normalized_server_url TEXT NOT NULL DEFAULT '',
			server_binding_state TEXT NOT NULL DEFAULT 'unbound'
				CHECK (server_binding_state IN ('unbound','binding','bound','switching_server','credential_revoked','unbinding')),
			owner_user_id TEXT NOT NULL DEFAULT '',
			client_id TEXT NOT NULL DEFAULT '',
			server_binding_revision INTEGER NOT NULL DEFAULT 0,
			config_signing_public_key BLOB,
			config_signing_key_id TEXT NOT NULL DEFAULT '',
			tls_trust_mode TEXT NOT NULL DEFAULT 'system'
				CHECK (tls_trust_mode IN ('system','pinned_spki','custom_ca')),
			pinned_spki_sha256 TEXT NOT NULL DEFAULT '',
			custom_ca_path TEXT NOT NULL DEFAULT '',
			desired_config_version INTEGER NOT NULL DEFAULT 0,
			applied_config_version INTEGER NOT NULL DEFAULT 0,
			last_failed_config_version INTEGER NOT NULL DEFAULT 0,
			last_sync_at TEXT NOT NULL DEFAULT '',
			last_heartbeat_at TEXT NOT NULL DEFAULT '',
			client_panel_version TEXT NOT NULL DEFAULT '',
			frpc_version TEXT NOT NULL DEFAULT '',
			protocol_version INTEGER NOT NULL DEFAULT 1,
			config_schema_version INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT OR IGNORE INTO client_installation (id, installation_instance_id) VALUES (1, '');`,
	},
	{
		Version: 2,
		Name:    "create_client_config_state",
		SQL: `CREATE TABLE IF NOT EXISTS client_config_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_version INTEGER NOT NULL,
			config_hash TEXT NOT NULL DEFAULT '',
			config_body TEXT NOT NULL DEFAULT '',
			signature_valid INTEGER NOT NULL DEFAULT 0,
			apply_status TEXT NOT NULL DEFAULT 'pending'
				CHECK (apply_status IN ('pending','applied','failed','rolled_back')),
			applied_at TEXT NOT NULL DEFAULT '',
			error_summary TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_config_state_version ON client_config_state(config_version);`,
	},
	{
		Version: 3,
		Name:    "create_frpc_runtime_state",
		SQL: `CREATE TABLE IF NOT EXISTS frpc_runtime_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			pid INTEGER NOT NULL DEFAULT 0,
			start_time_unix INTEGER NOT NULL DEFAULT 0,
			binary_path TEXT NOT NULL DEFAULT '',
			binary_hash TEXT NOT NULL DEFAULT '',
			binary_version TEXT NOT NULL DEFAULT '',
			admin_addr TEXT NOT NULL DEFAULT '',
			admin_user TEXT NOT NULL DEFAULT '',
			admin_pass_encrypted BLOB,
			status TEXT NOT NULL DEFAULT 'stopped'
				CHECK (status IN ('stopped','starting','running','stopping','error')),
			last_error TEXT NOT NULL DEFAULT '',
			uptime_seconds INTEGER NOT NULL DEFAULT 0,
			proxy_count INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT OR IGNORE INTO frpc_runtime_state (id) VALUES (1);`,
	},
	{
		Version: 4,
		Name:    "create_server_binding_aliases",
		SQL: `CREATE TABLE IF NOT EXISTS server_binding_aliases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alias_url TEXT NOT NULL UNIQUE,
			verified_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
	},
}

// RunMigrations 按版本顺序执行所有未应用的迁移
// 使用单独的迁移表跟踪已应用版本
func RunMigrations(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	// 创建迁移跟踪表
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("创建迁移跟踪表失败: %w", err)
	}

	for _, m := range clientMigrations {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.Version).Scan(&count)
		if err != nil {
			return fmt.Errorf("查询迁移版本 %d 状态失败: %w", m.Version, err)
		}
		if count > 0 {
			continue
		}

		logger.Info("执行数据库迁移", "version", m.Version, "name", m.Name)

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("开始迁移事务失败: %w", err)
		}

		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("执行迁移 %d (%s) 失败: %w", m.Version, m.Name, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("记录迁移版本 %d 失败: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %d 失败: %w", m.Version, err)
		}

		logger.Info("迁移完成", "version", m.Version, "name", m.Name)
	}

	return nil
}
