package parser

import (
	"bufio"
	"io"
	"strconv"

	"github.com/chenjiayao/goredistraning/redis/request"
)

func ReadCommand(reader io.Reader) chan request.RedisRequet {
	ch := make(chan request.RedisRequet)
	go ParseFromSocket(reader, ch)
	return ch
}

func ParseFromSocket(reader io.Reader, ch chan request.RedisRequet) {
	buf := bufio.NewReader(reader)

	for {
		cmds := [][]byte{}
		header, err := buf.ReadBytes('\n')
		if err != nil {

			ch <- request.RedisRequet{
				Err: err,
			}
			//如果是客户端关闭了，那么就不要读了，直接退出当前协程
			if io.EOF == err {
				break
			}
			continue
		}

		if header[0] != '*' {
			ch <- request.PROTOCOL_ERROR_REQUEST
			continue
		}
		argsCount, err := parseCmdArgsCount(header)
		if err != nil {
			ch <- request.PROTOCOL_ERROR_REQUEST
			continue
		}

		//依次读取 数组参数
		// *3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n
		readArgsFail := false
		for i := 0; i < argsCount; i++ {
			argsWithDelimiter, err := buf.ReadBytes('\n')
			if err != nil {
				ch <- request.RedisRequet{
					Err: err,
				}
				//如果是客户端关闭了，那么就不要读了，直接退出当前协程
				if io.EOF == err {
					break
				}
				readArgsFail = true
				break
			}

			// $3\r\n
			if argsWithDelimiter[0] != '$' ||
				argsWithDelimiter[len(argsWithDelimiter)-1] != '\n' ||
				argsWithDelimiter[len(argsWithDelimiter)-2] != '\r' {

				ch <- request.PROTOCOL_ERROR_REQUEST
				readArgsFail = true
				break
			}
			cmdLen, err := parseOneCmdArgsLen(argsWithDelimiter)
			if err != nil {
				ch <- request.PROTOCOL_ERROR_REQUEST
				readArgsFail = true
				break
			}

			cmd := make([]byte, cmdLen+2)
			_, err = io.ReadFull(buf, cmd)
			if err != nil {
				ch <- request.PROTOCOL_ERROR_REQUEST
				readArgsFail = true
				break
			}
			cmds = append(cmds, cmd[:len(cmd)-2])
		}

		if readArgsFail {
			continue
		}

		ch <- request.RedisRequet{
			Args: cmds,
		}
	}
}

//解析 header *3\r\n
func parseCmdArgsCount(header []byte) (int, error) {
	argsCountAsByte := header[1 : len(header)-2]

	argsCountAsStr := string(argsCountAsByte)
	argsCount, err := strconv.Atoi(argsCountAsStr)
	return argsCount, err
}

//$3\r\n
func parseOneCmdArgsLen(cmd []byte) (int, error) {
	cmdLenAsByte := cmd[1 : len(cmd)-2]
	cmdLenAsStr := string(cmdLenAsByte)
	argsCount, err := strconv.Atoi(cmdLenAsStr)
	return argsCount, err
}
