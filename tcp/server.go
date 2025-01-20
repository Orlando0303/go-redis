package tcp

import (
	"context"
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
	logger.Info("bind: %s, start listening...", cfg.Address)
	ListenAndServe(listener, handler, closeChan)
	return nil
}

func ListenAndServe(listener net.Listener, handler tcp.Handler, closeChan chan struct{}) {

	go func() {
		<-closeChan
		logger.Info("shutdown signal received")
		_ = listener.Close()
		_ = handler.Close()
	}()
	defer func() {
		logger.Info("closing listener")
		_ = listener.Close()
		_ = handler.Close()
	}()
	ctx := context.Background()
	var wg sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("accept err: %v", err)
			continue
		}
		logger.Info("new connection from: %s", conn.RemoteAddr())
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.Handle(ctx, conn)
		}()
	}
	wg.Wait()

}
