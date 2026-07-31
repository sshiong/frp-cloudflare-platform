// Package jobs 提供后台任务系统。
// 基于 SQLite 实现的任务队列，支持租约机制、心跳续期、去重、
// 指数退避重试和 worker 崩溃恢复。
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
)

// State 任务状态。
type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateDead      State = "dead"
)

// Job 任务实体。
type Job struct {
	ID          string `json:"id"`
	JobType     string `json:"job_type"`
	Payload     string `json:"payload"`
	State       State  `json:"state"`
	Priority    int    `json:"priority"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"max_attempts"`
	LeaseUntil  string `json:"lease_until"`
	LockedBy    string `json:"locked_by"`
	Error       string `json:"error"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Handler 任务处理函数。
type Handler func(ctx context.Context, job *Job) error

// Manager 任务管理器。
type Manager struct {
	db        *sql.DB
	logger    *slog.Logger
	workerID  string
	handlers  map[string]Handler
	leaseTTL  time.Duration
	pollDelay time.Duration
	stopCh    chan struct{}
}

// Config 任务管理器配置。
type Config struct {
	LeaseTTL  time.Duration // 租约有效期
	PollDelay time.Duration // 轮询间隔
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		LeaseTTL:  5 * time.Minute,
		PollDelay: 2 * time.Second,
	}
}

// NewManager 创建任务管理器。
func NewManager(db *sql.DB, logger *slog.Logger, cfg Config) *Manager {
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 5 * time.Minute
	}
	if cfg.PollDelay <= 0 {
		cfg.PollDelay = 2 * time.Second
	}
	return &Manager{
		db:        db,
		logger:    logger,
		workerID:  crypto.RandomToken(8),
		handlers:  make(map[string]Handler),
		leaseTTL:  cfg.LeaseTTL,
		pollDelay: cfg.PollDelay,
		stopCh:    make(chan struct{}),
	}
}

// Register 注册任务处理函数。
func (m *Manager) Register(jobType string, handler Handler) {
	m.handlers[jobType] = handler
}

// Enqueue 入队一个新任务。
func (m *Manager) Enqueue(ctx context.Context, jobType string, payload interface{}, priority int) (string, error) {
	payloadJSON := "{}"
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal payload: %w", err)
		}
		payloadJSON = string(b)
	}

	id := crypto.RandomToken(16)
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO jobs (id, job_type, payload, state, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, jobType, payloadJSON, string(StatePending), priority,
		time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return "", fmt.Errorf("insert job: %w", err)
	}
	return id, nil
}

// EnqueueDedup 入队一个去重任务。如果已存在相同类型和 payload 的 pending/running 任务则跳过。
func (m *Manager) EnqueueDedup(ctx context.Context, jobType string, payload interface{}, priority int) (string, bool, error) {
	payloadJSON := "{}"
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return "", false, fmt.Errorf("marshal payload: %w", err)
		}
		payloadJSON = string(b)
	}

	// 检查是否已存在
	var count int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM jobs
		WHERE job_type = ? AND payload = ? AND state IN ('pending', 'running')
	`, jobType, payloadJSON).Scan(&count)
	if err != nil {
		return "", false, fmt.Errorf("check duplicate: %w", err)
	}
	if count > 0 {
		return "", false, nil // 已存在，跳过
	}

	id, err := m.Enqueue(ctx, jobType, payload, priority)
	return id, true, err
}

// Start 启动 worker 循环。
func (m *Manager) Start(ctx context.Context) {
	m.logger.Info("job worker started", "worker_id", m.workerID)
	go m.recoverStaleJobs(ctx)
	go m.loop(ctx)
}

// Stop 停止 worker。
func (m *Manager) Stop() {
	close(m.stopCh)
	m.logger.Info("job worker stopped", "worker_id", m.workerID)
}

// loop 主轮询循环。
func (m *Manager) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
		}

		job, err := m.claim(ctx)
		if err != nil {
			m.logger.Error("claim job error", "err", err)
			time.Sleep(m.pollDelay)
			continue
		}
		if job == nil {
			time.Sleep(m.pollDelay)
			continue
		}

		m.process(ctx, job)
	}
}

// claim 获取一个待处理的任务（原子性）。
func (m *Manager) claim(ctx context.Context) (*Job, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 使用 SQLite 的写事务保证原子性
	var job Job
	var leaseUntil string
	err = tx.QueryRowContext(ctx, `
		SELECT id, job_type, payload, state, priority, attempts, max_attempts, lease_until, error, created_at, updated_at
		FROM jobs
		WHERE state = 'pending' AND (lease_until IS NULL OR lease_until < ?)
		ORDER BY priority DESC, created_at ASC
		LIMIT 1
	`, time.Now().UTC().Format(time.RFC3339)).Scan(
		&job.ID, &job.JobType, &job.Payload, &job.State, &job.Priority,
		&job.Attempts, &job.MaxAttempts, &leaseUntil, &job.Error, &job.CreatedAt, &job.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query next job: %w", err)
	}

	// 更新为 running 状态并设置租约
	leaseTime := time.Now().UTC().Add(m.leaseTTL)
	_, err = tx.ExecContext(ctx, `
		UPDATE jobs SET state = ?, lease_until = ?, locked_by = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = ? AND state = 'pending'
	`, string(StateRunning), leaseTime.Format(time.RFC3339), m.workerID,
		time.Now().UTC().Format(time.RFC3339), job.ID)
	if err != nil {
		return nil, fmt.Errorf("update job state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	job.State = StateRunning
	job.LeaseUntil = leaseTime.Format(time.RFC3339)
	job.LockedBy = m.workerID
	job.Attempts++
	return &job, nil
}

// process 处理一个任务。
func (m *Manager) process(ctx context.Context, job *Job) {
	handler, ok := m.handlers[job.JobType]
	if !ok {
		m.logger.Error("no handler for job type", "job_type", job.JobType, "job_id", job.ID)
		m.markDead(ctx, job.ID, fmt.Sprintf("no handler for type: %s", job.JobType))
		return
	}

	// 启动心跳续期
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go m.heartbeat(heartbeatCtx, job.ID)

	// 执行处理
	err := handler(ctx, job)
	if err != nil {
		m.logger.Error("job handler failed", "job_id", job.ID, "job_type", job.JobType, "err", err)
		m.handleFailure(ctx, job, err)
		return
	}

	// 成功
	_, _ = m.db.ExecContext(ctx, `
		UPDATE jobs SET state = ?, lease_until = NULL, locked_by = NULL, error = '', updated_at = ?
		WHERE id = ?
	`, string(StateSucceeded), time.Now().UTC().Format(time.RFC3339), job.ID)
	m.logger.Info("job completed", "job_id", job.ID, "job_type", job.JobType)
}

// heartbeat 定期续期任务租约。
func (m *Manager) heartbeat(ctx context.Context, jobID string) {
	ticker := time.NewTicker(m.leaseTTL / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaseTime := time.Now().UTC().Add(m.leaseTTL)
			_, _ = m.db.ExecContext(ctx, `
				UPDATE jobs SET lease_until = ? WHERE id = ? AND locked_by = ?
			`, leaseTime.Format(time.RFC3339), jobID, m.workerID)
		}
	}
}

// handleFailure 处理任务失败（指数退避 + 抖动）。
func (m *Manager) handleFailure(ctx context.Context, job *Job, err error) {
	if job.Attempts >= job.MaxAttempts {
		m.markDead(ctx, job.ID, err.Error())
		return
	}

	// 指数退避: base=2^attempts 秒，最大 1 小时，加随机抖动
	backoffSec := math.Pow(2, float64(job.Attempts))
	if backoffSec > 3600 {
		backoffSec = 3600
	}
	jitter := rand.Float64() * backoffSec * 0.1 // 10% 抖动
	nextRetry := time.Now().UTC().Add(time.Duration(backoffSec+jitter) * time.Second)

	_, _ = m.db.ExecContext(ctx, `
		UPDATE jobs SET state = 'pending', lease_until = ?, locked_by = NULL, error = ?, updated_at = ?
		WHERE id = ?
	`, nextRetry.Format(time.RFC3339), err.Error(), time.Now().UTC().Format(time.RFC3339), job.ID)
}

// markDead 将任务标记为 dead。
func (m *Manager) markDead(ctx context.Context, jobID, errMsg string) {
	_, _ = m.db.ExecContext(ctx, `
		UPDATE jobs SET state = ?, lease_until = NULL, locked_by = NULL, error = ?, updated_at = ?
		WHERE id = ?
	`, string(StateDead), errMsg, time.Now().UTC().Format(time.RFC3339), jobID)
	m.logger.Error("job marked as dead", "job_id", jobID, "error", errMsg)
}

// recoverStaleJobs 恢复因 worker 崩溃而停滞的任务。
func (m *Manager) recoverStaleJobs(ctx context.Context) {
	// 等待一小段时间确保数据库连接稳定
	time.Sleep(5 * time.Second)

	result, err := m.db.ExecContext(ctx, `
		UPDATE jobs SET state = 'pending', lease_until = NULL, locked_by = NULL, updated_at = ?
		WHERE state = 'running' AND lease_until < ?
	`, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		m.logger.Error("recover stale jobs failed", "err", err)
		return
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		m.logger.Info("recovered stale jobs", "count", n)
	}
}

// GetJob 获取任务详情。
func (m *Manager) GetJob(ctx context.Context, id string) (*Job, error) {
	var job Job
	var leaseUntil, lockedBy sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, job_type, payload, state, priority, attempts, max_attempts, lease_until, locked_by, error, created_at, updated_at
		FROM jobs WHERE id = ?
	`, id).Scan(&job.ID, &job.JobType, &job.Payload, &job.State, &job.Priority,
		&job.Attempts, &job.MaxAttempts, &leaseUntil, &lockedBy, &job.Error, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if leaseUntil.Valid {
		job.LeaseUntil = leaseUntil.String
	}
	if lockedBy.Valid {
		job.LockedBy = lockedBy.String
	}
	return &job, nil
}

// ListJobs 列出任务。
func (m *Manager) ListJobs(ctx context.Context, state State, limit, offset int) ([]Job, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	countSQL := "SELECT COUNT(*) FROM jobs"
	var countArgs []interface{}
	if state != "" {
		countSQL += " WHERE state = ?"
		countArgs = append(countArgs, string(state))
	}
	if err := m.db.QueryRowContext(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataSQL := "SELECT id, job_type, payload, state, priority, attempts, max_attempts, COALESCE(lease_until,''), COALESCE(locked_by,''), error, created_at, updated_at FROM jobs"
	var dataArgs []interface{}
	if state != "" {
		dataSQL += " WHERE state = ?"
		dataArgs = append(dataArgs, string(state))
	}
	dataSQL += " ORDER BY priority DESC, created_at ASC LIMIT ? OFFSET ?"
	dataArgs = append(dataArgs, limit, offset)

	rows, err := m.db.QueryContext(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.JobType, &j.Payload, &j.State, &j.Priority,
			&j.Attempts, &j.MaxAttempts, &j.LeaseUntil, &j.LockedBy, &j.Error, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, j)
	}
	return jobs, total, nil
}
