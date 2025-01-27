package parser

import (
	"go-redis/interface/resp"
	"io"
)

type Payload struct {
	Data resp.Reply
	Err  error
}

type readState struct {
	//解析单行还是多行数据
	readingMultiLine bool
	//该cmd应该有几个参数 记录是否解析完
	expectedArgsCount int

	msgType byte
	//已经解析的参数
	args [][]byte
	//字节组长度
	bulkLen int64
}

func (s *readState) finished() bool {
	return s.expectedArgsCount > 0 && len(s.args) == s.expectedArgsCount
}

func ParseStream(reader io.Reader) <-chan *Payload {
	ch := make(chan *Payload)
	go parse0(reader, ch)
	return ch
}

func parse0(reader io.Reader, ch chan<- *Payload) {

}
