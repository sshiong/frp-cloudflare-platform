// Package operations 提供操作跟踪功能。
// 管理操作的状态机、阶段推进、补偿重试和取消支持。
package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/frp-panel/server-panel/internal/crypto"
)

// State 操作状态。
type State string

const (
	StatePending         State = "pending"
	StateRunning         State = "running"
	StateWaitingClient   State = "waiting_client"
	StateWaitingExternal State = "waiting_external"
	StateSucceeded       State = "succeeded"
	StateFailed          State = "failed"
	StateCancelled       State = "cancelled"
)

// 合法的状态转换
var validTransitions = map[State][]State{
	StatePending:         {StateRunning, StateCancelled},
	StateRunning:         {StateWaitingClient, StateWaitingExternal, StateSucceeded, StateFailed},
	StateWaitingClient:   {StateRunning, StateCancelled},
	StateWaitingExternal: {StateRunning, StateFailed},
	// 终态不可转换
	StateSucceeded: {},
	StateFailed:    {StateRunning}, // 允许重试
	StateCancelled: {},
}

// Operation 操作实体。
type Operation struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	OpType     string                 `json:"op_type"`
	TargetID   string                 `json:"target_id"`
	TargetType string                 `json:"target_type"`
	State      State                  `json:"state"`
	Phase      string                 `json:"phase"`
	Progress   int                    `json:"progress"`
	Error      string                 `json:"error"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  string                 `json:"created_at"`
	UpdatedAt  string                 `json:"updated_at"`
}

// Manager 操作管理器。
type Manager struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewManager 创建操作管理器。
func NewManager(db *sql.DB, logger *slog.Logger) *Manager {
	return &Manager{db: db, logger: logger}
}

// Create 创建新操作。
func (m *Manager) Create(ctx context.Context, userID, opType, targetID, targetType string, metadata map[string]interface{}) (*Operation, error) {
	id := crypto.RandomToken(16)
	metaJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metaJSON = string(b)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO operations (id, user_id, op_type, target_id, target_type, state, phase, progress, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'init', 0, ?, ?, ?)
	`, id, userID, opType, targetID, targetType, string(StatePending), metaJSON, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert operation: %w", err)
	}

	return &Operation{
		ID:         id,
		UserID:     userID,
		OpType:     opType,
		TargetID:   targetID,
		TargetType: targetType,
		State:      StatePending,
		Phase:      "init",
		Progress:   0,
		Metadata:   metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Get 获取操作。
func (m *Manager) Get(ctx context.Context, id string) (*Operation, error) {
	var op Operation
	var metaJSON string
	err := m.db.QueryRowContext(ctx, `
		SELECT id, user_id, op_type, target_id, target_type, state, phase, progress, error, metadata, created_at, updated_at
		FROM operations WHERE id = ?
	`, id).Scan(&op.ID, &op.UserID, &op.OpType, &op.TargetID, &op.TargetType,
		&op.State, &op.Phase, &op.Progress, &op.Error, &metaJSON, &op.CreatedAt, &op.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query operation: %w", err)
	}
	op.Metadata = make(map[string]interface{})
	_ = json.Unmarshal([]byte(metaJSON), &op.Metadata)
	return &op, nil
}

// UpdateState 更新操作状态。
func (m *Manager) UpdateState(ctx context.Context, id string, newState State) error {
	op, err := m.Get(ctx, id)
	if err != nil {
		return err
	}
	if op == nil {
		return fmt.Errorf("operation %s not found", id)
	}

	allowed, ok := validTransitions[op.State]
	if !ok {
		return fmt.Errorf("no transitions defined for state %s", op.State)
	}
	valid := false
	for _, s := range allowed {
		if s == newState {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid transition: %s -> %s", op.State, newState)
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE operations SET state = ?, updated_at = ? WHERE id = ?
	`, string(newState), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// UpdatePhase 更新操作阶段和进度。
func (m *Manager) UpdatePhase(ctx context.Context, id, phase string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	_, err := m.db.ExecContext(ctx, `
		UPDATE operations SET phase = ?, progress = ?, updated_at = ? WHERE id = ?
	`, phase, progress, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// SetError 标记操作失败并记录错误信息。
func (m *Manager) SetError(ctx context.Context, id, errMsg string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE operations SET state = ?, error = ?, updated_at = ? WHERE id = ?
	`, string(StateFailed), errMsg, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// Cancel 取消操作（仅允许在可逆阶段取消）。
func (m *Manager) Cancel(ctx context.Context, id string) error {
	return m.UpdateState(ctx, id, StateCancelled)
}

// Retry 重试失败的操作。
func (m *Manager) Retry(ctx context.Context, id string) error {
	return m.UpdateState(ctx, id, StateRunning)
}

// ListByUser 列出用户的操作。
func (m *Manager) ListByUser(ctx context.Context, userID string, limit, offset int) ([]Operation, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operations WHERE user_id = ?", userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := m.db.QueryContext(ctx, `
		SELECT id, user_id, op_type, target_id, target_type, state, phase, progress, error, metadata, created_at, updated_at
		FROM operations WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ops []Operation
	for rows.Next() {
		var op Operation
		var metaJSON string
		if err := rows.Scan(&op.ID, &op.UserID, &op.OpType, &op.TargetID, &op.TargetType,
			&op.State, &op.Phase, &op.Progress, &op.Error, &metaJSON, &op.CreatedAt, &op.UpdatedAt); err != nil {
			return nil, 0, err
		}
		op.Metadata = make(map[string]interface{})
		_ = json.Unmarshal([]byte(metaJSON), &op.Metadata)
		ops = append(ops, op)
	}
	return ops, total, nil
}

// ForceComplete 强制完成操作（管理员用）。
func (m *Manager) ForceComplete(ctx context.Context, id string, success bool) error {
	state := StateSucceeded
	if !success {
		state = StateFailed
	}
	_, err := m.db.ExecContext(ctx, `
		UPDATE operations SET state = ?, progress = 100, updated_at = ? WHERE id = ?
	`, string(state), time.Now().UTC().Format(time.RFC3339), id)
	return err
}
