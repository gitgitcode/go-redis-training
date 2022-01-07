package redis

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/chenjiayao/goredistraning/config"
	"github.com/chenjiayao/goredistraning/interface/response"
	"github.com/chenjiayao/goredistraning/interface/server"
	"github.com/chenjiayao/goredistraning/lib/atomic"
	"github.com/chenjiayao/goredistraning/lib/logger"
	"github.com/chenjiayao/goredistraning/parser"
	"github.com/chenjiayao/goredistraning/redis/resp"
)

var _ server.Server = &RedisServer{}

// handler 实例只会有一个
type RedisServer struct {
	closed     atomic.Boolean
	rds        *RedisDBs
	aofHandler *AofHandler
}

///////////启动 redis 服务，
// 如果这里有 aof，那么需要加载 aof
func MakeRedisServer() *RedisServer {
	redisServer := &RedisServer{
		closed: atomic.Boolean(0),
	}

	redisServer.rds = NewDBs()
	redisServer.aofHandler = MakeAofHandler(redisServer)
	return redisServer
}

func ListenAndServe(server server.Server) {

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.Config.Bind, config.Config.Port))
	if err != nil {
		logger.Fatal("start listen failed : ", err)
	}

	logger.Info(fmt.Sprintf("start listen %s", listener.Addr().String()))
	if config.Config.Appendonly {
		server.Log()
	}

	defer func() {
		listener.Close()
		server.Close()
	}()

	var waitGroup sync.WaitGroup

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		logger.Info("accept link")
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			server.Handle(conn)
		}()
	}

	//这里使用 waitGroup 的作用是：还有 conn 在处理情况下
	// 如果 redis server 关闭，那么这里需要 wait 等待已有链接处理完成。
	waitGroup.Wait()
}

func (redisServer *RedisServer) Log() {
	redisServer.aofHandler.StartAof()
}

func (redisServer *RedisServer) Handle(conn net.Conn) {

	if redisServer.closed.Get() {
		conn.Close()
	}

	redisClient := MakeRedisConn(conn)

	ch := parser.ReadCommand(conn)
	//chan close 掉之后， range 直接退出
	for request := range ch {
		if request.Err != nil {
			if request.Err == io.EOF {
				redisServer.closeClient(redisClient)
				return
			}

			errResponse := resp.MakeErrorResponse(request.Err.Error())
			err := redisClient.Write(errResponse.ToErrorByte()) //返回执行命令失败，close client
			if err != nil {
				logger.Info("response failed: " + redisClient.RemoteAddress())
				redisServer.closeClient(redisClient)
				return
			}
		}

		var res response.Response

		cmd := request.Args
		cmdName := redisServer.parseCommand(request.Args)
		args := cmd[1:]
		if cmdName == "auth" {
			res = redisServer.auth(redisClient, args)
			err := redisServer.sendResponse(redisClient, res)
			if err == io.EOF {
				redisServer.closeClient(redisClient)
				break
			}
			continue
		}

		if !redisServer.isAuthenticated(redisClient) {
			res := resp.MakeErrorResponse("NOAUTH Authentication required")
			err := redisServer.sendResponse(redisClient, res)
			if err == io.EOF {
				redisServer.closeClient(redisClient)
				break
			}
			continue
		}
		//执行 select 命令
		if cmdName == "select" {
			dbStr := string(args[0])
			index, err := strconv.Atoi(dbStr)
			if err != nil {
				redisServer.sendResponse(redisClient, resp.MakeErrorResponse("ERR invalid DB index"))
				if err == io.EOF {
					redisServer.closeClient(redisClient)
					break
				}
			}

			redisClient.SetSelectedDBIndex(index)

			res = resp.MakeSimpleResponse("OK")
			err = redisServer.sendResponse(redisClient, res)
			redisServer.aofHandler.LogCmd(request.Args)
			if err == io.EOF {
				redisServer.closeClient(redisClient)
				break
			}
			continue
		}

		selectedDBIndex := redisClient.GetSelectedDBIndex()
		selectedDB := redisServer.rds.DBs[selectedDBIndex]

		res = selectedDB.Exec(cmdName, args)
		err := redisServer.sendResponse(redisClient, res)
		if res.ISOK() {
			redisServer.aofHandler.LogCmd(request.Args)
		}
		if err == io.EOF {
			redisServer.closeClient(redisClient)
			break
		}
	}
}

func (redisServer *RedisServer) isAuthenticated(redisClient *RedisConn) bool {
	return config.Config.RequirePass == redisClient.GetPassword()
}

func (redisServer *RedisServer) sendResponse(redisClient *RedisConn, res response.Response) error {
	var err error
	if _, ok := res.(resp.RedisErrorResponse); ok {
		err = redisClient.Write(res.ToErrorByte())
	} else {
		err = redisClient.Write(res.ToContentByte())
	}
	return err
}

func (redisServer *RedisServer) auth(c *RedisConn, args [][]byte) response.Response {
	if config.Config.RequirePass == "" {
		return resp.MakeErrorResponse("ERR Client sent AUTH, but no password is set")
	}

	if len(args) != 1 {
		return resp.MakeErrorResponse("ERR wrong number of arguments for 'auth' command")
	}
	password := string(args[0])
	if config.Config.RequirePass != password {
		return resp.MakeErrorResponse("ERR invalid password")
	}
	c.SetPassword(password)
	return resp.MakeSimpleResponse("ok")
}

//从请求数据中解析出 redis 命令
func (redisServer *RedisServer) parseCommand(cmd [][]byte) string {
	cmdName := string(cmd[0])
	return strings.ToLower(cmdName)
}

// closeClient
func (redisServer *RedisServer) closeClient(client *RedisConn) {
	logger.Info(fmt.Sprintf("client %s closed", client.RemoteAddress()))
	client.Close()
}

func (redisServer *RedisServer) Close() error {
	logger.Info("server close....")
	redisServer.aofHandler.EndAof()
	return nil
}
