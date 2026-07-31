// Package storage 提供 SQLite 数据库连接管理和迁移功能。
// 使用 WAL 模式、外键约束和优化的连接池配置。
package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Config 数据库配置。
type Config struct {
	Path         string        // 数据库文件路径
	MaxOpenConns int           // 最大打开连接数
	MaxIdleConns int           // 最大空闲连接数
	ConnMaxLife  time.Duration // 连接最大存活时间
}

// DefaultConfig 返回默认配置。
func DefaultConfig(path string) Config {
	return Config{
		Path:         path,
		MaxOpenConns: 1, // SQLite 单写者模型，写连接只能一个
		MaxIdleConns: 1,
		ConnMaxLife:  0, // 永不回收（SQLite 文件锁管理）
	}
}

// Open 打开 SQLite 数据库连接并执行初始化设置。
func Open(cfg Config) (*sql.DB, error) {
	// 确保数据库目录存在
	dir := filepath.Dir(cfg.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	// DSN 参数：
	//   _journal_mode=WAL     - 写前日志模式，提升并发读性能
	//   _foreign_keys=ON      - 启用外键约束
	//   _busy_timeout=5000    - 忙等待超时 5 秒
	//   _synchronous=FULL     - 最高同步级别，保证数据安全
	//   _auto_vacuum=INCREMENTAL - 增量回收已删除页面空间
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000&_synchronous=FULL&_auto_vacuum=INCREMENTAL",
		cfg.Path)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	if cfg.ConnMaxLife > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLife)
	}

	// 验证连接可用
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	// 执行 WAL 模式下的 PRAGMA 设置（必须在每次连接建立时执行）
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=FULL",
		"PRAGMA auto_vacuum=INCREMENTAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("execute %s: %w", p, err)
		}
	}

	return db, nil
}

// RunMigrations 从指定目录读取 SQL 迁移文件并按顺序执行。
// 使用 migrations 表跟踪已执行的迁移。
func RunMigrations(db *sql.DB, migrationsDir string, logger *slog.Logger) error {
	// 创建迁移跟踪表
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// 读取迁移目录
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("migrations directory not found, skipping", "dir", migrationsDir)
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// 收集并排序迁移文件（只处理 .sql 文件）
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") && strings.HasPrefix(e.Name(), "0") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, filename := range files {
		// 检查是否已执行
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", filename).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if count > 0 {
			logger.Debug("migration already applied, skipping", "file", filename)
			continue
		}

		// 读取文件内容
		content, err := os.ReadFile(filepath.Join(migrationsDir, filename))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		// 分离 Up 和 Down 语句（只执行 Up 部分）
		upSQL := extractUpSQL(string(content))
		if upSQL == "" {
			logger.Warn("no up migration found in file", "file", filename)
			continue
		}

		// 在事务中执行迁移
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction for %s: %w", filename, err)
		}

		if _, err := tx.Exec(upSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (filename) VALUES (?)", filename); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", filename, err)
		}

		logger.Info("applied migration", "file", filename)
	}

	return nil
}

// extractUpSQL 从迁移文件中提取 Up（正向迁移）SQL。
// 支持 "+migrate Up" / "+migrate Down" 标记格式。
func extractUpSQL(content string) string {
	lines := strings.Split(content, "\n")
	var upLines []string
	inUp := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "+migrate Up") {
			inUp = true
			continue
		}
		if strings.Contains(trimmed, "+migrate Down") {
			break
		}
		if inUp {
			upLines = append(upLines, line)
		}
	}

	// 如果没有标记，默认整个文件就是 Up
	if !inUp {
		return content
	}

	return strings.Join(upLines, "\n")
}

// WrapInTransaction 在事务中执行 fn。
// 如果 fn 返回错误，事务回滚；否则提交。
func WrapInTransaction(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
