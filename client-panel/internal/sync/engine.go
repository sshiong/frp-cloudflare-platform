package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ConfigVersion 配置版本信息
type ConfigVersion struct {
	Version     int64  `json:"version"`
	Hash        string `json:"hash"`
	Signature   string `json:"signature"`
	KeyID       string `json:"key_id"`
	SchemaVer   int    `json:"schema_version"`
	GeneratedAt string `json:"generated_at"`
}

// SyncState 同步状态
type SyncState struct {
	LastSyncAt          time.Time
	LastSyncedVersion   int64
	LastSyncError       string
	IsSyncing           bool
	FullSyncCount       int
	IncrementalSyncCount int
}

// Syncer 同步接口，供外部实现
type Syncer interface {
	// PullFullConfig 拉取完整配置
	PullFullConfig(ctx context.Context) (json.RawMessage, *ConfigVersion, error)
	// ReportApplyResult 上报应用结果
	ReportApplyResult(ctx context.Context, version int64, success bool, errorSummary string) error
}

// Applier 配置应用接口
type Applier interface {
	// ApplyConfig 应用配置
	ApplyConfig(ctx context.Context, config json.RawMessage, version *ConfigVersion) error
	// GetCurrentVersion 获取当前已应用版本
	GetCurrentVersion() int64
}

// Engine 同步引擎
// 管理配置同步的全生命周期
// 负责版本比较、配置拉取和应用协调
type Engine struct {
	mu          sync.Mutex
	syncer      Syncer
	applier     Applier
	state       SyncState
	logger      *slog.Logger
	triggerCh   chan struct{}
	stopCh      chan struct{}
}

// NewEngine 创建同步引擎
func NewEngine(syncer Syncer, applier Applier, logger *slog.Logger) *Engine {
	return &Engine{
		syncer:    syncer,
		applier:   applier,
		logger:    logger,
		triggerCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动同步引擎
func (e *Engine) Start(ctx context.Context) {
	e.logger.Info("同步引擎已启动")

	// 启动后立即执行一次全量同步
	go e.performFullSync(ctx)

	// 监听触发信号
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopCh:
				return
			case <-e.triggerCh:
				e.performFullSync(ctx)
			}
		}
	}()
}

// Stop 停止同步引擎
func (e *Engine) Stop() {
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}
	e.logger.Info("同步引擎已停止")
}

// TriggerSync 触发同步
// WebSocket 收到 config_update 等事件后调用
func (e *Engine) TriggerSync() {
	select {
	case e.triggerCh <- struct{}{}:
	default:
		// 已有等待中的触发，忽略
	}
}

// performFullSync 执行全量同步
// 流程：
// 1. 拉取完整配置
// 2. 验证版本
// 3. 应用配置
// 4. 上报结果
func (e *Engine) performFullSync(ctx context.Context) {
	e.mu.Lock()
	if e.state.IsSyncing {
		e.mu.Unlock()
		return
	}
	e.state.IsSyncing = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.state.IsSyncing = false
		e.mu.Unlock()
	}()

	e.logger.Info("开始全量同步")

	// 拉取配置
	config, version, err := e.syncer.PullFullConfig(ctx)
	if err != nil {
		e.recordSyncError(err)
		return
	}

	// 版本比较
	currentVersion := e.applier.GetCurrentVersion()
	if version.Version <= currentVersion {
		e.logger.Info("配置版本未变化，跳过应用",
			"server_version", version.Version,
			"local_version", currentVersion,
		)
		e.recordSyncSuccess(version.Version)
		return
	}

	e.logger.Info("检测到新配置版本",
		"server_version", version.Version,
		"local_version", currentVersion,
	)

	// 应用配置
	if err := e.applier.ApplyConfig(ctx, config, version); err != nil {
		e.recordSyncError(err)

		// 上报失败
		if reportErr := e.syncer.ReportApplyResult(ctx, version.Version, false, err.Error()); reportErr != nil {
			e.logger.Warn("上报应用失败结果失败", "error", reportErr)
		}
		return
	}

	// 上报成功
	if err := e.syncer.ReportApplyResult(ctx, version.Version, true, ""); err != nil {
		e.logger.Warn("上报应用成功结果失败", "error", err)
	}

	e.recordSyncSuccess(version.Version)
}

// recordSyncError 记录同步错误
func (e *Engine) recordSyncError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.LastSyncError = err.Error()
	e.state.LastSyncAt = time.Now()
	e.logger.Error("同步失败", "error", err)
}

// recordSyncSuccess 记录同步成功
func (e *Engine) recordSyncSuccess(version int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.LastSyncedVersion = version
	e.state.LastSyncAt = time.Now()
	e.state.LastSyncError = ""
	e.state.FullSyncCount++
	e.logger.Info("同步成功", "version", version)
}

// GetState 获取同步状态
func (e *Engine) GetState() SyncState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// CompareVersions 比较版本
// 返回值：-1 (a < b), 0 (a == b), 1 (a > b)
func CompareVersions(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
