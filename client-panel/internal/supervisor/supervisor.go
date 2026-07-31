package supervisor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Supervisor FRPC 父进程/监督者
// 监控 FRPC 进程健康状态
// 自动重启意外退出的 FRPC
// 捕获和管理日志
type Supervisor struct {
	mu           sync.Mutex
	manager      FRPCManager
	configPath   string
	logger       *slog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	enabled      bool
	restartCount int
	maxRestarts  int
	restartDelay time.Duration
	healthTicker *time.Ticker
}

// FRPCManager FRPC 管理器接口
type FRPCManager interface {
	Start(ctx context.Context, configPath string) error
	Stop() error
	GetStatus() string
	GetProcessInfo() (pid int, startTime time.Time, binaryHash string)
}

// NewSupervisor 创建监督者
func NewSupervisor(manager FRPCManager, configPath string, logger *slog.Logger) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		manager:      manager,
		configPath:   configPath,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		maxRestarts:  5,               // 最大连续重启次数
		restartDelay: 5 * time.Second, // 重启间隔
	}
}

// Start 启动监督者
func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = true
	s.restartCount = 0

	s.logger.Info("FRPC 监督者已启动")

	// 启动 FRPC
	if err := s.manager.Start(s.ctx, s.configPath); err != nil {
		return err
	}

	// 启动健康监控
	s.healthTicker = time.NewTicker(10 * time.Second)
	go s.monitor()

	return nil
}

// Stop 停止监督者
func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = false
	s.cancel()

	if s.healthTicker != nil {
		s.healthTicker.Stop()
	}

	s.logger.Info("FRPC 监督者已停止")
}

// monitor 持续监控 FRPC 状态
func (s *Supervisor) monitor() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.healthTicker.C:
			s.checkHealth()
		}
	}
}

// checkHealth 检查 FRPC 健康状态
func (s *Supervisor) checkHealth() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return
	}

	status := s.manager.GetStatus()

	switch status {
	case "running":
		// 正常运行，重置重启计数
		s.restartCount = 0
		s.logger.Debug("FRPC 健康检查通过")

	case "error", "stopped":
		// 异常退出，尝试重启
		s.logger.Warn("FRPC 异常退出，尝试重启",
			"status", status,
			"restart_count", s.restartCount,
		)

		if s.restartCount >= s.maxRestarts {
			s.logger.Error("FRPC 连续重启次数超限，停止监督",
				"max_restarts", s.maxRestarts,
			)
			s.enabled = false
			return
		}

		// 等待一段时间后重启
		time.Sleep(s.restartDelay)

		if err := s.manager.Start(s.ctx, s.configPath); err != nil {
			s.logger.Error("FRPC 自动重启失败", "error", err)
			s.restartCount++
		} else {
			s.restartCount++
			s.logger.Info("FRPC 已自动重启", "attempt", s.restartCount)
		}

	case "starting", "stopping":
		// 过渡状态，等待下次检查
		s.logger.Debug("FRPC 过渡状态", "status", status)
	}
}

// GetRestartCount 获取当前连续重启次数
func (s *Supervisor) GetRestartCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartCount
}

// IsEnabled 检查监督者是否启用
func (s *Supervisor) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// ResetRestartCount 重置重启计数（手动恢复后调用）
func (s *Supervisor) ResetRestartCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartCount = 0
	s.enabled = true
}
