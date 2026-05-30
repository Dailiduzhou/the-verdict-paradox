package server

import (
	"context"
	"time"

	"github.com/Dailiduzhou/the-verdict-paradox/backend/app/game/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

// 确保 MatchServer 实现了 transport.Server 接口
var _ transport.Server = (*MatchServer)(nil)

type MatchServer struct {
	matchUsecase *biz.MatchUsecase
	log          *log.Helper
	tickInterval time.Duration

	stop chan struct{}
}

func NewMatchServer(mu *biz.MatchUsecase, logger log.Logger) *MatchServer {
	return &MatchServer{
		matchUsecase: mu,
		log:          log.NewHelper(logger),
		tickInterval: time.Second * 1, // 每秒轮询一次
		stop:         make(chan struct{}),
	}
}

// Start Kratos 启动时会自动调用此方法
func (s *MatchServer) Start(ctx context.Context) error {
	s.log.Infof("匹配服务引擎已启动，轮询间隔: %v", s.tickInterval)

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			execCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			s.matchUsecase.LockAndMatch(execCtx, 6)
			cancel()

		case <-s.stop:
			s.log.Info("接收到停止信号，匹配循环已安全退出")
			return nil
		}
	}
}

// Stop Kratos 收到关闭信号（如Ctrl+C，K8s销毁Pod）时会自动调用此方法
func (s *MatchServer) Stop(ctx context.Context) error {
	s.log.Info("正在关闭匹配服务引擎...")
	close(s.stop) // 触发 Start 中的 s.stop 分支，退出循环

	// 在这里可以做一些清理工作，比如把处于异常状态的队列数据重置
	return nil
}
