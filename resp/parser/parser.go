package parser

import (
	"bufio"
	"errors"
	"go-redis/interface/resp"
	"go-redis/lib/logger"
	"go-redis/resp/reply"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
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
	//状态位
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
	defer func() {
		if err := recover(); err != nil {
			logger.Error(string(debug.Stack()))
		}
	}()
	bufReader := bufio.NewReader(reader)
	var state readState
	for {
		// read line
		msg, ioErr, err := readLine(bufReader, &state)
		if err != nil {
			if ioErr {
				ch <- &Payload{Err: err}
				close(ch)
				return
			}
			// 处理协议错误时，跳过无效数据
			discardInvalidData(bufReader) // 新增跳过无效数据的逻辑
			ch <- &Payload{Err: err}
			state = readState{}
			continue
		}

		// parse line
		/*
			*：多行批量回复（multi bulk reply）
			$：批量回复（bulk reply）
			其他字符：单行回复（single line reply）
		*/
		if !state.readingMultiLine { // 用来设置状态机 或 读取单行回复+ - :
			// receive new response
			if msg[0] == '*' {
				// multi bulk reply
				err = parseMultiBulkHeader(msg, &state) // 解析协议头 设置状态机 state.readingMultiLine = true
				if err != nil {
					ch <- &Payload{
						Err: err,
					}
					state = readState{} // reset state
					continue
				}
				if state.expectedArgsCount == 0 { // 参数数量为0 直接返回空reply
					ch <- &Payload{
						Data: reply.MakeEmptyMultiBulkReply(),
					}
					state = readState{} // reset state
					continue
				}
			} else if msg[0] == '$' { // bulk reply
				err = parseBulkHeader(msg, &state) // 解析协议头 设置状态机 state.readingMultiLine = true
				if err != nil {
					ch <- &Payload{
						Err: err,
					}
					state = readState{} // reset state
					continue
				}
				if state.bulkLen == -1 { // 空字符串
					ch <- &Payload{
						Data: reply.MakeNullBulkReply(),
					}
					state = readState{} // reset state
					continue
				}
			} else {
				// single line reply
				result, err := parseSingleLineReply(msg)
				ch <- &Payload{
					Data: result,
					Err:  err,
				}
				state = readState{} // reset state
				continue
			}
		} else {
			// receive following bulk reply
			err = readBody(msg, &state)
			if err != nil {
				ch <- &Payload{
					Err: err,
				}
				state = readState{} // reset state
				continue
			}
			// if sending finished
			if state.finished() {
				var result resp.Reply
				if state.msgType == '*' {
					result = reply.MakeMultiBulkReply(state.args)
				} else if state.msgType == '$' {
					result = reply.MakeBulkReply(state.args[0])
				}
				ch <- &Payload{
					Data: result,
					Err:  err,
				}
				state = readState{}
			}
		}
	}
}

// 读取当前行 直到\n 返回msg，是否io error，error
func readLine(bufReader *bufio.Reader, state *readState) ([]byte, bool, error) {
	var msg []byte
	var err error
	//单行协议 + - ：，或者*3\r\n $5\r\n协议头
	if state.bulkLen == 0 { // read normal line
		msg, err = bufReader.ReadBytes('\n') //存储到第一次遇到\n
		// io错误
		if err != nil {
			return nil, true, err
		}
		// 协议错误
		if len(msg) == 0 || msg[len(msg)-2] != '\r' {
			return nil, false, errors.New("protocol error: " + string(msg))
		}
	} else { // read bulk line (binary safe)
		msg = make([]byte, state.bulkLen+2)  // 存储 一个字符串\r\n
		_, err = io.ReadFull(bufReader, msg) // 可能包含任意二进制数据（包括 \r\n），如果使用 ReadBytes，可能会错误地将数据截断
		if err != nil {
			return nil, true, err
		}
		if len(msg) == 0 ||
			msg[len(msg)-2] != '\r' ||
			msg[len(msg)-1] != '\n' ||
			int64(len(msg)-2) != state.bulkLen {
			return nil, false, errors.New("protocol error: " + string(msg))
		}
		state.bulkLen = 0
	}
	return msg, false, nil
}

/*
*3\r\n
 */
func parseMultiBulkHeader(msg []byte, state *readState) error {
	var err error
	var expectedLine uint64
	// 读取元素（参数）个数
	expectedLine, err = strconv.ParseUint(string(msg[1:len(msg)-2]), 10, 32)
	if err != nil {
		return errors.New("protocol error: " + string(msg))
	}
	if expectedLine == 0 {
		state.expectedArgsCount = 0
		return nil
	} else if expectedLine > 0 { // 有参数 设置状态机
		// first line of multi bulk reply
		state.msgType = msg[0]                       // 消息类型为数组：*
		state.readingMultiLine = true                // 多行消息
		state.expectedArgsCount = int(expectedLine)  // 元素（参数）个数
		state.args = make([][]byte, 0, expectedLine) // 初始化切片 为即将解析的参数分配内存
		return nil
	} else {
		return errors.New("protocol error: " + string(msg))
	}
}

/*$5\r\n  */
func parseBulkHeader(msg []byte, state *readState) error {
	var err error
	// 读取字符个数 赋给状态机bulkLen
	state.bulkLen, err = strconv.ParseInt(string(msg[1:len(msg)-2]), 10, 64)
	if err != nil {
		return errors.New("protocol error: " + string(msg))
	}
	if state.bulkLen == -1 { // null bulk
		return nil
	} else if state.bulkLen > 0 {
		state.msgType = msg[0]            // 消息类型为字符串：$
		state.readingMultiLine = true     // 多行消息（协议头+字符串）
		state.expectedArgsCount = 1       // 单行字符串
		state.args = make([][]byte, 0, 1) // 初始化切片 为即将解析的字符串分配内存
		return nil
	} else {
		return errors.New("protocol error: " + string(msg))
	}
}

/*
+OK\r\n          // 简单字符串
-Error msg\r\n   // 错误类型
:1000\r\n        // 整数
*/
func parseSingleLineReply(msg []byte) (resp.Reply, error) {
	str := strings.TrimSuffix(string(msg), "\r\n") //保留\r\n之前字符串
	var result resp.Reply
	switch msg[0] {
	case '+': // status reply
		result = reply.MakeStatusReply(str[1:]) //保留+之后
	case '-': // err reply
		result = reply.MakeErrReply(str[1:]) //保留-之后
	case ':': // int reply
		val, err := strconv.ParseInt(str[1:], 10, 64) //字符串型数字转为整型
		if err != nil {
			return nil, errors.New("protocol error: " + string(msg))
		}
		result = reply.MakeIntReply(val)
	}
	return result, nil
}

// read the non-first lines of multi bulk reply or bulk reply
/*
hello\r\n (单行字符串）
$3\r\nset\r\n$3\r\nkey\r\n$5\r\nvalue\r\n （数组 多行参数）读进来时只有 $3\r\n
*/
func readBody(msg []byte, state *readState) error {
	line := msg[0 : len(msg)-2] // 去掉最后的\r\n
	var err error
	if line[0] == '$' { //数组的第一个参数 $3
		// bulk reply
		state.bulkLen, err = strconv.ParseInt(string(line[1:]), 10, 64)
		if err != nil {
			return errors.New("protocol error: " + string(msg))
		}
		if state.bulkLen <= 0 { // null bulk in multi bulks
			state.args = append(state.args, []byte{})
			state.bulkLen = 0
		}
	} else { //单行字符串 hello
		state.args = append(state.args, line) //填入状态机已解析参数
	}
	return nil
}

// discardInvalidData 跳过无效数据直到找到下一个RESP命令头
func discardInvalidData(bufReader *bufio.Reader) {
	for {
		b, err := bufReader.ReadByte()
		if err != nil {
			return // 遇到I/O错误或EOF则退出
		}
		// 检查是否是合法的RESP类型起始字符
		if b == '*' || b == '$' || b == '+' || b == '-' || b == ':' {
			bufReader.UnreadByte() // 将合法字符放回缓冲区
			return
		}
	}
}
