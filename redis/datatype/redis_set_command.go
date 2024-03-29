package datatype

import (
	"github.com/chenjiayao/goredistraning/interface/conn"
	"github.com/chenjiayao/goredistraning/interface/response"
	"github.com/chenjiayao/goredistraning/lib/set"
	"github.com/chenjiayao/goredistraning/redis"
	"github.com/chenjiayao/goredistraning/redis/resp"
	"github.com/chenjiayao/goredistraning/redis/validate"
)

// TODO set 中很多操作达不到 redis 的时间复杂度，这里先做功能实现，后续再考虑性能优化

/**
SADD
SCARD
SMEMBERS
SISMEMBER


SDIFF
SDIFFSTORE
SINTER
SINTERSTORE
SMOVE
SPOP
SRANDMEMBER
SREM
SUNION
SUNIONSTORE
SSCAN
*/
func init() {
	redis.RegisterExecCommand(redis.Sdiff, ExecSdiff, validate.ValidateSdiff)
	redis.RegisterExecCommand(redis.Sismember, ExecSismember, validate.ValidateSismember)
	redis.RegisterExecCommand(redis.Spop, ExecSpop, validate.ValidateSpop)
	redis.RegisterExecCommand(redis.Sadd, ExecSadd, validate.ValidateSadd)
	redis.RegisterExecCommand(redis.Scard, ExecScard, validate.ValidateScard)
	redis.RegisterExecCommand(redis.Smembers, ExecSmembers, validate.ValidateSmembers)

}

const (
	size = 2 >> 64
)

func ExecSdiff(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	return nil
}

func ExecSismember(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	key := string(args[0])
	s := getSet(conn, db, key)

	if s == nil {
		return resp.MakeNumberResponse(0)
	}

	exist := s.Exist(key)
	if exist {
		return resp.MakeNumberResponse(1)
	}
	return resp.MakeNumberResponse(0)
}

func ExecSpop(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	key := string(args[0])
	s := getSet(conn, db, key)
	if s == nil {
		return resp.NullMultiResponse
	}

	return nil
}

//SADD runoobkey redis
func ExecSadd(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	setValue := getSetOrInitSet(conn, db, string(args[0]))

	for _, v := range args[1:] {
		setValue.Add(string(v))
	}

	db.Dataset.Put(string(args[0]), setValue)
	return resp.MakeNumberResponse(1)
}

func ExecScard(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	key := string(args[0])
	s := getSetOrInitSet(conn, db, key)
	return resp.MakeNumberResponse(int64(s.Len()))
}

func ExecSmembers(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	setValue := getSetOrInitSet(conn, db, string(args[0]))
	if setValue.Len() == 0 {
		// TODO 返回空数组
		// return resp.make
		return resp.MakeArrayResponse(nil)
	}

	members := setValue.Members()
	simpleResponses := make([]response.Response, len(members))
	for i := 0; i < len(members); i++ {
		simpleResponses[i] = resp.MakeSimpleResponse(string(members[i]))
	}
	return resp.MakeArrayResponse(simpleResponses)
}

//如果 key 不存在，会新建一个 set
func getSetOrInitSet(conn conn.Conn, db *redis.RedisDB, key string) *set.Set {
	s := getSet(conn, db, key)
	if s == nil {
		return set.MakeSet(size)
	}
	return s
}

func getSet(conn conn.Conn, db *redis.RedisDB, key string) *set.Set {
	d, exist := db.Dataset.Get(key)
	if exist {
		setValue := d.(*set.Set)
		return setValue
	}
	return nil
}
