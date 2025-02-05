package tcp

import (
	"context"
	"errors"
	"go-redis/interface/tcp"
	"go-redis/lib/logger"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Config struct {
	Address string
}

func ListenAndServeWithSignal(cfg *Config, handler tcp.Handler) error {
	closeChan := make(chan struct{})
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		switch <-signals {
		case syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			closeChan <- struct{}{}
		}
	}()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		panic(err)
	}
	logger.Info("start listening:", cfg.Address)
	ListenAndServe(listener, handler, closeChan)
	return nil
}

func ListenAndServe(listener net.Listener, handler tcp.Handler, closeChan chan struct{}) {

	// 使用 context 控制循环退出
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-closeChan
		logger.Info("shutdown signal received")
		cancel()
		_ = listener.Close()
		_ = handler.Close()
	}()

	defer func() {
		logger.Info("closing listener")
		_ = listener.Close()
		logger.Info("server fully stopped")
	}()

	var wg sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			// 关键：检查是否为监听器关闭导致的错误
			if errors.Is(err, net.ErrClosed) {
				break // 退出循环
			}
			logger.Error("accept err:", err)
			continue
		}
		logger.Info("new connection from:", conn.RemoteAddr())
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.Handle(ctx, conn)
		}()
	}
	wg.Wait()

}
