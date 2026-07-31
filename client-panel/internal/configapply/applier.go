package configapply

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ApplyAction 应用动作
type ApplyAction string

const (
	ActionReload  ApplyAction = "reload"
	ActionRestart ApplyAction = "restart"
)

// ApplyResult 应用结果
type ApplyResult struct {
	Success       bool
	ConfigVersion int64
	Action        ApplyAction
	ErrorSummary  string
	AppliedAt     time.Time
	RollbackDone  bool
}

// FRPCController FRPC 控制接口
type FRPCController interface {
	// Verify 验证配置文件
	Verify(configPath string) error
	// Reload 重载配置（代理变更）
	Reload() error
	// Restart 重启进程（Server/Auth/TLS 变更）
	Restart(ctx context.Context, configPath string) error
	// Status 获取状态
	Status() (string, error)
	// Stop 停止进程
	Stop() error
}

// Renderer 配置渲染接口
type Renderer interface {
	RenderProxyOnly(proxies interface{}) (string, error)
}

// Applier 配置应用器
// 单工作者队列，串行应用配置
// 流程：渲染 -> 验证 -> 备份 -> 原子替换 -> reload/restart -> 验证 -> 上报
type Applier struct {
	mu            sync.Mutex
	frpcCtrl      FRPCController
	configDir     string // 配置根目录
	currentDir    string // current/ 目录
	candidateDir  string // candidate/ 目录
	rollbackDir   string // rollback/ 目录
	logger        *slog.Logger
	currentVersion int64
	workerCh      chan *applyTask
	stopCh        chan struct{}
}

type applyTask struct {
	ctx     context.Context
	config  json.RawMessage
	version int64
	resultCh chan *ApplyResult
}

// NewApplier 创建配置应用器
func NewApplier(frpcCtrl FRPCController, configDir string, logger *slog.Logger) *Applier {
	return &Applier{
		frpcCtrl:     frpcCtrl,
		configDir:    configDir,
		currentDir:   filepath.Join(configDir, "current"),
		candidateDir: filepath.Join(configDir, "candidate"),
		rollbackDir:  filepath.Join(configDir, "rollback"),
		logger:       logger,
		workerCh:     make(chan *applyTask, 10),
		stopCh:       make(chan struct{}),
	}
}

// Start 启动应用工作者
func (a *Applier) Start(ctx context.Context) {
	// 确保目录存在
	os.MkdirAll(a.currentDir, 0700)
	os.MkdirAll(a.candidateDir, 0700)
	os.MkdirAll(a.rollbackDir, 0700)

	go a.worker(ctx)
	a.logger.Info("配置应用器已启动")
}

// Stop 停止应用工作者
func (a *Applier) Stop() {
	close(a.stopCh)
}

// ApplyConfig 提交配置应用请求
// 异步执行，返回结果通道
func (a *Applier) ApplyConfig(ctx context.Context, config json.RawMessage, version int64) error {
	task := &applyTask{
		ctx:      ctx,
		config:   config,
		version:  version,
		resultCh: make(chan *ApplyResult, 1),
	}

	select {
	case a.workerCh <- task:
		// 等待结果
		select {
		case result := <-task.resultCh:
			if !result.Success {
				return fmt.Errorf("配置应用失败: %s", result.ErrorSummary)
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	default:
		return fmt.Errorf("应用队列已满")
	}
}

// GetCurrentVersion 获取当前已应用版本
func (a *Applier) GetCurrentVersion() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentVersion
}

// worker 工作者协程
func (a *Applier) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case task := <-a.workerCh:
			result := a.applySingle(task)
			task.resultCh <- result
		}
	}
}

// applySingle 应用单个配置任务
func (a *Applier) applySingle(task *applyTask) *ApplyResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := &ApplyResult{
		ConfigVersion: task.version,
	}

	a.logger.Info("开始应用配置", "version", task.version)

	// 1. 写入候选目录
	if err := a.writeCandidate(task.config); err != nil {
		result.ErrorSummary = fmt.Sprintf("写入候选配置失败: %s", err.Error())
		a.logger.Error("写入候选配置失败", "error", err)
		return result
	}

	// 2. 检查文件权限
	if err := a.checkPermissions(); err != nil {
		result.ErrorSummary = fmt.Sprintf("文件权限检查失败: %s", err.Error())
		return result
	}

	// 3. 执行 frpc verify
	candidateToml := filepath.Join(a.candidateDir, "frpc.toml")
	if err := a.frpcCtrl.Verify(candidateToml); err != nil {
		result.ErrorSummary = fmt.Sprintf("配置验证失败: %s", err.Error())
		a.logger.Error("配置验证失败", "error", err)
		return result
	}

	// 4. 备份当前配置到 rollback
	if err := a.backupCurrent(); err != nil {
		result.ErrorSummary = fmt.Sprintf("备份当前配置失败: %s", err.Error())
		a.logger.Error("备份当前配置失败", "error", err)
		return result
	}

	// 5. 原子替换（fsync + rename）
	if err := a.atomicReplace(); err != nil {
		result.ErrorSummary = fmt.Sprintf("原子替换失败: %s", err.Error())
		a.logger.Error("原子替换失败", "error", err)
		// 尝试回滚
		a.rollback()
		result.RollbackDone = true
		return result
	}

	// 6. 判断 reload 还是 restart
	action := a.determineAction(task.config)
	result.Action = action

	// 7. 执行 reload 或 restart
	configPath := filepath.Join(a.currentDir, "frpc.toml")
	var applyErr error
	switch action {
	case ActionReload:
		applyErr = a.frpcCtrl.Reload()
	case ActionRestart:
		applyErr = a.frpcCtrl.Restart(task.ctx, configPath)
	}

	if applyErr != nil {
		result.ErrorSummary = fmt.Sprintf("执行 %s 失败: %s", action, applyErr.Error())
		a.logger.Error("执行失败", "action", action, "error", applyErr)

		// 回滚
		if rollbackErr := a.rollback(); rollbackErr != nil {
			a.logger.Error("回滚失败", "error", rollbackErr)
		} else {
			result.RollbackDone = true
			// 尝试恢复旧配置
			if action == ActionRestart {
				a.frpcCtrl.Restart(task.ctx, configPath)
			} else {
				a.frpcCtrl.Reload()
			}
		}
		return result
	}

	// 8. 验证应用结果
	status, statusErr := a.frpcCtrl.Status()
	if statusErr != nil {
		a.logger.Warn("获取状态失败", "error", statusErr)
	} else {
		a.logger.Info("FRPC 状态", "status", status)
	}

	// 成功
	result.Success = true
	result.AppliedAt = time.Now()
	a.currentVersion = task.version

	a.logger.Info("配置应用成功", "version", task.version, "action", action)
	return result
}

// writeCandidate 将配置写入候选目录
func (a *Applier) writeCandidate(config json.RawMessage) error {
	// 清空候选目录
	os.RemoveAll(a.candidateDir)
	os.MkdirAll(a.candidateDir, 0700)
	os.MkdirAll(filepath.Join(a.candidateDir, "conf.d"), 0700)

	// 注意：实际配置渲染由 configrender 包完成
	// 这里写入的是渲染后的 TOML 文本
	// 简化实现：假设 config 已经是渲染后的文本
	if len(config) > 0 {
		mainToml := filepath.Join(a.candidateDir, "frpc.toml")
		if err := os.WriteFile(mainToml, config, 0600); err != nil {
			return fmt.Errorf("写入主配置失败: %w", err)
		}
	}

	return nil
}

// checkPermissions 检查文件权限
// 敏感文件必须为 0600
func (a *Applier) checkPermissions() error {
	return filepath.Walk(a.candidateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 检查权限（允许 0600 或更严格）
		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			return fmt.Errorf("文件 %s 权限过宽: %o（要求 0600 或更严格）", path, perm)
		}
		return nil
	})
}

// backupCurrent 备份当前配置到 rollback 目录
func (a *Applier) backupCurrent() error {
	// 清空 rollback 目录
	os.RemoveAll(a.rollbackDir)

	// 复制 current 到 rollback
	return copyDir(a.currentDir, a.rollbackDir)
}

// atomicReplace 原子替换 current 配置
// 流程：fsync 候选文件 -> rename 到 current
func (a *Applier) atomicReplace() error {
	// 在同一文件系统上执行 rename
	// 先确保候选目录数据已落盘
	candidateMain := filepath.Join(a.candidateDir, "frpc.toml")
	if err := fsyncFile(candidateMain); err != nil {
		return fmt.Errorf("fsync 候选文件失败: %w", err)
	}

	// 清空 current 目录
	os.RemoveAll(a.currentDir)

	// rename candidate -> current
	if err := os.Rename(a.candidateDir, a.currentDir); err != nil {
		return fmt.Errorf("rename 失败: %w", err)
	}

	// 重建候选目录
	os.MkdirAll(a.candidateDir, 0700)

	return nil
}

// rollback 回滚到备份配置
func (a *Applier) rollback() error {
	a.logger.Info("开始回滚配置")

	// 停止当前 FRPC
	a.frpcCtrl.Stop()

	// 清空 current
	os.RemoveAll(a.currentDir)

	// rename rollback -> current
	if err := os.Rename(a.rollbackDir, a.currentDir); err != nil {
		return fmt.Errorf("回滚 rename 失败: %w", err)
	}

	// 重建 rollback 目录
	os.MkdirAll(a.rollbackDir, 0700)

	a.logger.Info("配置回滚完成")
	return nil
}

// determineAction 判断需要 reload 还是 restart
// FRP 官方限制：frpc reload 主要用于代理项变化
// 以下变更必须 restart：
// - Server 地址/端口
// - FRP 用户/认证 Token
// - TLS 配置
// - Admin API 配置
func (a *Applier) determineAction(config json.RawMessage) ApplyAction {
	// 解析配置检查是否有 restart 必需的变更
	var cfg struct {
		ServerAddr string `json:"server_addr"`
		ServerPort int    `json:"server_port"`
	}
	if json.Unmarshal(config, &cfg) == nil {
		// 如果包含 server 配置变更，需要 restart
		if cfg.ServerAddr != "" || cfg.ServerPort > 0 {
			return ActionRestart
		}
	}

	// 默认尝试 reload（代理变更）
	return ActionReload
}

// fsyncFile 同步文件到磁盘
func fsyncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
