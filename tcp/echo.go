package tcp

import (
	"context"
	"go-redis/lib/sync/atomic"
	"go-redis/lib/sync/wait"
	"net"
	"sync"
	"time"
)

type EchoClient struct {
	conn    net.Conn
	waiting wait.Wait
}

func (client *EchoClient) close() error {
	client.waiting.WaitWithTimeout(10 * time.Second)
	client.conn.Close()
	return nil
}

type EchoHandler struct {
	activeConn sync.Map
	closing    atomic.Boolean
}

func (handler *EchoHandler) Handle(ctx context.Context, conn net.Conn) {
	if handler.closing.Get() {
		_ = conn.Close()
	}
	client := &EchoClient{conn: conn}
	handler.activeConn.Store(client, struct{}{})
}

func (handler *EchoHandler) Close() error {
	//TODO implement me
	panic("implement me")
}
