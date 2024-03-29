package datatype

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/chenjiayao/goredistraning/helper"
	"github.com/chenjiayao/goredistraning/interface/conn"
	"github.com/chenjiayao/goredistraning/interface/response"
	"github.com/chenjiayao/goredistraning/redis"
	"github.com/chenjiayao/goredistraning/redis/rediserr"
	"github.com/chenjiayao/goredistraning/redis/resp"
	"github.com/chenjiayao/goredistraning/redis/validate"
)

// - set
// - get
// - incr
// - incrby
// - decr
// - decrby
// - incrbyfloat
// - getset
// - psetex
// - setnx
// - setex
// - mset
// - mget
// - msetnx

func init() {

	redis.RegisterExecCommand(redis.Set, ExecSet, validate.ValidateSet)
	redis.RegisterExecCommand(redis.Mget, ExecMGet, validate.ValidateMGet)
	redis.RegisterExecCommand(redis.Get, ExecGet, validate.ValidateGet)
	redis.RegisterExecCommand(redis.Incr, ExecIncr, validate.ValidateIncr)
	redis.RegisterExecCommand(redis.Incrby, ExecIncrBy, validate.ValidateIncrBy)
	redis.RegisterExecCommand(redis.Incrbyf, ExecIncrByFloat, validate.ValidateIncreByFloat)
	redis.RegisterExecCommand(redis.Getset, ExecGetset, validate.ValidateGetSet)
	redis.RegisterExecCommand(redis.Psetex, ExecPSetEX, validate.ValidatePSetEx)
	redis.RegisterExecCommand(redis.Setnx, ExecSetNX, validate.ValidateSetNx)
	redis.RegisterExecCommand(redis.Setex, ExecSetEX, validate.ValidateSetEx)
	redis.RegisterExecCommand(redis.Mset, ExecMSet, validate.ValidateMSet)
	redis.RegisterExecCommand(redis.Mget, ExecMGet, validate.ValidateMGet)
	redis.RegisterExecCommand(redis.Msetnx, ExecMSetNX, validate.ValidateMSetNX)
}

func ExecMSet(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	for i := 0; i < len(args); i += 2 {
		ExecSet(conn, db, [][]byte{
			args[i],
			args[i+1],
		})
	}
	return resp.OKSimpleResponse
}

func ExecMGet(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	res := make([][]byte, 0)
	for i := 0; i < len(args); i++ {
		r := getAsString(conn, db, args[i])
		if r == "" {
			res = append(res, nil)
		} else {
			res = append(res, []byte(r))
		}
	}
	return resp.MakeMultiResponse(res)
}

func ExecMSetNX(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	//  这里加锁之前应该对 args 按照字母顺序排序， 保证每个goroutine 的 keys 加锁顺序一致，不然会导致死锁
	allKeys := make([]string, 0)
	for i := 0; i < len(args); i += 2 {
		allKeys = append(allKeys, string(args[i]))
	}
	sort.Slice(allKeys, func(i, j int) bool { return i < j })

	//对所有的 key 加锁
	for i := 0; i < len(allKeys); i++ {
		key := allKeys[i]
		db.LockKey(key)
		defer db.UnLockKey(key)
	}

	//检查是否有哪个 key 已经存在
	for i := 0; i < len(args); i += 2 {
		s := getAsString(conn, db, args[i])
		if s != "" {
			return resp.MakeNumberResponse(0)
		}
	}

	for i := 0; i < len(args); i++ {
		ExecSet(conn, db, [][]byte{
			args[i],
			args[i+1],
		})
	}
	return resp.MakeNumberResponse(1)
}

func ExecGetset(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	key := string(args[0])

	db.LockKey(key)
	defer db.UnLockKey(key)

	i, exists := db.Dataset.Get(key)

	if !exists {
		return resp.NullMultiResponse
	}

	res, ok := i.(string)
	if !ok {
		return resp.MakeErrorResponse("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	db.Dataset.PutIfExist(key, string(args[1]))
	return resp.MakeSimpleResponse(res)
}

// key value [EX seconds] [PX milliseconds] [NX|XX]
// NX -- Only set the key if it does not already exist.
// XX -- Only set the key if it already exist.   --->同时覆盖新的 ttl
func ExecSet(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	ttl := UnlimitTTL
	ttls := fmt.Sprintf("%d", ttl)

	ss := helper.BbyteToSString(args)
	exFlagIndex := helper.ContainWithoutCaseSensitive(ss, "EX")
	if exFlagIndex != -1 {
		ttlStr := string(args[exFlagIndex+1])
		ttli, _ := strconv.ParseInt(ttlStr, 10, 64)
		ttls = fmt.Sprintf("%d", ttli*1000)
	}

	pxFlagIndex := helper.ContainWithoutCaseSensitive(ss, "PX")
	if pxFlagIndex != -1 {
		ttls = string(args[pxFlagIndex+1])
	}

	key := string(args[0])
	value := string(args[1])

	//不存在 key 就 insert
	if helper.ContainWithoutCaseSensitive(ss, "NX") != -1 {
		ok := db.Dataset.PutIfNotExist(key, value)
		if ok {
			SetKeyTTL(conn, db, [][]byte{
				args[0],
				[]byte(ttls),
			})
		} else {
			return resp.NullMultiResponse
		}
	}

	//不存在key就 insert
	if helper.ContainWithoutCaseSensitive(ss, "XX") != -1 {
		ok := db.Dataset.PutIfExist(key, value)
		if ok {
			SetKeyTTL(conn, db, [][]byte{
				args[0],
				[]byte(ttls),
			})
			return resp.OKSimpleResponse
		} else {
			return resp.NullMultiResponse
		}
	}

	ok := db.Dataset.Put(key, value)

	if ok {
		ExecTTL(conn, db, [][]byte{
			args[0],
			[]byte(ttls),
		})
		return resp.OKSimpleResponse
	}
	return resp.NullMultiResponse
}

// setnx key value ---> set key value nx
func ExecSetNX(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	args = append(args, []byte("nx"))
	return ExecSet(conn, db, args)
}

// setex key seconds value ---> set key value ex second
func ExecSetEX(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	setArgs := [][]byte{
		args[0],
		args[2],
		[]byte("ex"),
		args[1],
	}
	return ExecSet(conn, db, setArgs)
}

// psetex key milliseconds value --> set key value px milliseconds
func ExecPSetEX(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	setArgs := [][]byte{
		args[0],
		args[2],
		[]byte("px"),
		args[1],
	}
	return ExecSet(conn, db, setArgs)
}

/**
get 执行之前要考虑 redis 的过期策略
	redis 的过期策略分为两种方式
		1. 定期删除：每次间隔一定时间再 ttlDict 中扫描，清除过期的 key
		2. 惰性删除：访问一个 key 之前，判断是否已经过期，如果已经过期那么直接删除，并且返回 null
*/
func ExecGet(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	//key 不存在，或者已经到过期时间了
	if ExecTTL(conn, db, [][]byte{args[0]}) < -1 {
		// TODO 删除 key
		return resp.NullMultiResponse
	}

	s := getAsString(conn, db, args[0])
	if s == "" {
		return resp.NullMultiResponse
	}
	return resp.MakeSimpleResponse(s)
}

func getAsString(conn conn.Conn, db *redis.RedisDB, key []byte) string {
	res, ok := db.Dataset.Get(string(key))
	if !ok {
		return ""
	}

	commo, ok := res.(string)
	if !ok {
		return ""
	}
	return commo
}

//如果 incr 的key 不存在，那么 set 为1
func ExecIncr(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	incrByArgs := append(args, []byte("1"))
	return ExecIncrBy(conn, db, incrByArgs)
}

func ExecIncrBy(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	key := string(args[0])
	steps := string(args[1])
	step, _ := strconv.ParseInt(steps, 10, 64)

	db.LockKey(string(args[0]))
	defer db.UnLockKey(key)

	//get
	s := getAsString(conn, db, args[0])

	val := ""
	//incr
	if s == "" {
		val = fmt.Sprint(step)
	} else {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return resp.MakeErrorResponse(rediserr.NOT_INTEGER_ERROR.Error()) //试图incr 一个字符串
		}
		v += step
		val = fmt.Sprint(v)
	}

	//set
	db.Dataset.Put(key, val)

	return resp.MakeSimpleResponse(val)
}
func ExecIncrByFloat(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	key := string(args[0])
	steps := string(args[1])
	step, _ := strconv.ParseFloat(steps, 64)

	db.LockKey(key)
	defer db.UnLockKey(key)

	//get
	s := getAsString(conn, db, args[0])

	val := ""
	//incr
	if s == "" {
		val = fmt.Sprint(step)
	} else {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return resp.MakeErrorResponse(rediserr.NOT_INTEGER_ERROR.Error()) //试图incr 一个字符串
		}
		v += step
		val = fmt.Sprint(v)
	}

	//set
	db.Dataset.Put(key, val)

	return resp.MakeSimpleResponse(val)
}

func ExecDecr(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {
	incrByArgs := append(args, []byte("-1"))
	return ExecIncrBy(conn, db, incrByArgs)
}

func ExecDecrBy(conn conn.Conn, db *redis.RedisDB, args [][]byte) response.Response {

	step := string(args[1])
	step = fmt.Sprintf("-%s", step) // 变成 -
	incrByArgs := [][]byte{
		args[0],
		[]byte(step),
	}
	return ExecIncrBy(conn, db, incrByArgs)
}
