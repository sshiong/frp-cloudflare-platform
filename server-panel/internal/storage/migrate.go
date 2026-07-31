package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration represents a single migration file.
type Migration struct {
	Version  int
	Name     string
	SQL      string
}

// Migrator manages database schema migrations.
type Migrator struct {
	db         *sql.DB
	migrations []Migration
}

// NewMigrator creates a new Migrator that reads embedded migration files
// and is ready to run against the given database connection.
func NewMigrator(db *sql.DB) (*Migrator, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return &Migrator{
		db:         db,
		migrations: migrations,
	}, nil
}

// loadMigrations reads all .sql files from the embedded migrations/ directory,
// parses their version numbers from the filename prefix, and returns them
// sorted by version.
func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		version, err := parseVersion(name)
		if err != nil {
			return nil, fmt.Errorf("parse version from %q: %w", name, err)
		}

		data, err := fs.ReadFile(migrationFS, filepath.Join("migrations", name))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(data),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parseVersion extracts the leading numeric version from a migration filename.
// For example, "001_create_system_identity.sql" returns 1.
func parseVersion(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("expected format NNN_name.sql, got %q", name)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid version number %q: %w", parts[0], err)
	}
	if version <= 0 {
		return 0, fmt.Errorf("version must be positive, got %d", version)
	}
	return version, nil
}

// ensureSchemaMigrationsTable creates the schema_migrations tracking table
// if it does not already exist.
func (m *Migrator) ensureSchemaMigrationsTable() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    applied_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    dirty       INTEGER NOT NULL DEFAULT 0
);`
	_, err := m.db.Exec(ddl)
	return err
}

// Up applies all pending migrations in order. Each migration runs inside
// its own transaction. The function is safe to call multiple times; already
// applied migrations are skipped. Returns the number of migrations applied.
func (m *Migrator) Up() (int, error) {
	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return 0, fmt.Errorf("get applied versions: %w", err)
	}

	count := 0
	for _, mig := range m.migrations {
		if applied[mig.Version] {
			continue
		}

		if err := m.applyMigration(mig); err != nil {
			return count, fmt.Errorf("apply migration %03d (%s): %w", mig.Version, mig.Name, err)
		}
		count++
	}
	return count, nil
}

// UpTo applies migrations up to and including the specified target version.
// Returns the number of migrations applied.
func (m *Migrator) UpTo(target int) (int, error) {
	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return 0, fmt.Errorf("get applied versions: %w", err)
	}

	count := 0
	for _, mig := range m.migrations {
		if mig.Version > target {
			break
		}
		if applied[mig.Version] {
			continue
		}

		if err := m.applyMigration(mig); err != nil {
			return count, fmt.Errorf("apply migration %03d (%s): %w", mig.Version, mig.Name, err)
		}
		count++
	}
	return count, nil
}

// Down rolls back the last N applied migrations in reverse order.
// Each rollback runs inside its own transaction.
// Returns the number of migrations rolled back.
func (m *Migrator) Down(steps int) (int, error) {
	if steps <= 0 {
		return 0, fmt.Errorf("steps must be positive, got %d", steps)
	}

	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	versions, err := m.appliedVersionsSorted()
	if err != nil {
		return 0, fmt.Errorf("get applied versions: %w", err)
	}

	if len(versions) == 0 {
		return 0, nil
	}

	// Roll back from highest version downward.
	if steps > len(versions) {
		steps = len(versions)
	}

	rollback := versions[len(versions)-steps:]
	// Reverse order.
	for i, j := 0, len(rollback)-1; i < j; i, j = i+1, j-1 {
		rollback[i], rollback[j] = rollback[j], rollback[i]
	}

	count := 0
	for _, v := range rollback {
		if err := m.rollbackVersion(v); err != nil {
			return count, fmt.Errorf("rollback migration %03d: %w", v, err)
		}
		count++
	}
	return count, nil
}

// applyMigration runs a single migration inside a transaction and records
// it in schema_migrations.
func (m *Migrator) applyMigration(mig Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		// Rollback is a no-op if already committed.
		_ = tx.Rollback()
	}()

	// Mark as dirty before applying.
	_, err = tx.Exec(
		"INSERT INTO schema_migrations (version, name, dirty) VALUES (?, ?, 1)",
		mig.Version, mig.Name,
	)
	if err != nil {
		return fmt.Errorf("record dirty migration: %w", err)
	}

	// Execute the migration SQL. We use ExecContext-safe multi-statement
	// execution by splitting on semicolons at the top level.
	if err := execSQLBatch(tx, mig.SQL); err != nil {
		return fmt.Errorf("execute SQL: %w", err)
	}

	// Mark as clean.
	_, err = tx.Exec(
		"UPDATE schema_migrations SET dirty = 0 WHERE version = ?",
		mig.Version,
	)
	if err != nil {
		return fmt.Errorf("clear dirty flag: %w", err)
	}

	return tx.Commit()
}

// rollbackVersion removes the record from schema_migrations.
// The actual DROP TABLE / undo SQL is not embedded; callers must handle
// destructive rollbacks explicitly. This only removes the tracking record
// so the migration can be re-applied.
func (m *Migrator) rollbackVersion(version int) error {
	_, err := m.db.Exec(
		"DELETE FROM schema_migrations WHERE version = ?",
		version,
	)
	return err
}

// appliedVersions returns a set of already-applied (non-dirty) migration versions.
func (m *Migrator) appliedVersions() (map[int]bool, error) {
	rows, err := m.db.Query(
		"SELECT version FROM schema_migrations WHERE dirty = 0",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// appliedVersionsSorted returns applied versions in ascending order.
func (m *Migrator) appliedVersionsSorted() ([]int, error) {
	rows, err := m.db.Query(
		"SELECT version FROM schema_migrations WHERE dirty = 0 ORDER BY version ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// CurrentVersion returns the highest applied migration version, or 0 if
// no migrations have been applied.
func (m *Migrator) CurrentVersion() (int, error) {
	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return 0, err
	}

	var version sql.NullInt64
	err := m.db.QueryRow(
		"SELECT MAX(version) FROM schema_migrations WHERE dirty = 0",
	).Scan(&version)
	if err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

// PendingCount returns the number of migrations that have not yet been applied.
func (m *Migrator) PendingCount() (int, error) {
	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return 0, err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, mig := range m.migrations {
		if !applied[mig.Version] {
			count++
		}
	}
	return count, nil
}

// List returns all known migrations with their applied status.
func (m *Migrator) List() ([]MigrationStatus, error) {
	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return nil, err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return nil, err
	}

	var result []MigrationStatus
	for _, mig := range m.migrations {
		result = append(result, MigrationStatus{
			Version: mig.Version,
			Name:    mig.Name,
			Applied: applied[mig.Version],
		})
	}
	return result, nil
}

// MigrationStatus reports whether a specific migration has been applied.
type MigrationStatus struct {
	Version int
	Name    string
	Applied bool
}

// execSQLBatch splits a SQL script on semicolons and executes each statement.
// This handles the common pattern of multi-statement migration files in SQLite.
func execSQLBatch(tx *sql.Tx, script string) error {
	// Split on semicolons. This is a simplified splitter that works for
	// the migration files in this project (no string literals containing
	// semicolons, no stored procedures).
	statements := splitSQL(script)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			// Truncate the statement in the error for readability.
			short := stmt
			if len(short) > 120 {
				short = short[:120] + "..."
			}
			return fmt.Errorf("exec %q: %w", short, err)
		}
	}
	return nil
}

// splitSQL splits a SQL script into individual statements by semicolons,
// ignoring semicolons inside single-quoted strings and line comments.
func splitSQL(script string) []string {
	var (
		stmts   []string
		current strings.Builder
		inQuote bool
		inLine  bool
	)

	runes := []rune(script)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Handle line comments.
		if !inQuote && !inLine && ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			inLine = true
			current.WriteRune(ch)
			continue
		}
		if inLine {
			current.WriteRune(ch)
			if ch == '\n' {
				inLine = false
			}
			continue
		}

		// Handle single-quoted strings.
		if ch == '\'' {
			if inQuote {
				// Check for escaped quote ('').
				if i+1 < len(runes) && runes[i+1] == '\'' {
					current.WriteRune(ch)
					current.WriteRune(runes[i+1])
					i++
					continue
				}
				inQuote = false
			} else {
				inQuote = true
			}
			current.WriteRune(ch)
			continue
		}

		if ch == ';' && !inQuote {
			stmts = append(stmts, current.String())
			current.Reset()
			continue
		}

		current.WriteRune(ch)
	}

	// Capture any trailing statement without semicolon.
	if s := strings.TrimSpace(current.String()); s != "" {
		stmts = append(stmts, s)
	}

	return stmts
}
