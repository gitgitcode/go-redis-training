package datatype

import (
	"bytes"
	"testing"

	"github.com/chenjiayao/goredistraning/lib/dict"
	"github.com/chenjiayao/goredistraning/redis"
	"github.com/chenjiayao/goredistraning/redis/resp"
)

func TestExecSet(t *testing.T) {
	db := redis.NewDBInstance(0)

	args := [][]byte{
		[]byte("key"),
		[]byte("value"),
	}
	got := ExecSet(db, args)
	want := resp.OKSimpleResponse
	if got != want {
		t.Errorf(" ExecSet(db, args) = %v, want = %v", got, want)
	}

	v, ok := db.Dataset.Get("key")
	if !ok {
		t.Errorf("execSet failed")
	}
	res := v.(string)
	if res != "value" {
		t.Errorf("set store value, but got = %s", res)
	}

	ttl := ExecTTL(db, [][]byte{[]byte("key")})
	if ttl != -1 {
		t.Errorf("set key  ttl = -1, but got = %d", ttl)
	}
}

func TestExecGet(t *testing.T) {
	db := redis.NewDBInstance(0)
	key := "key"
	value := "value"
	db.Dataset.Put(key, value)

	res := ExecGet(db, [][]byte{
		[]byte(key),
	})

	want := string(resp.MakeSimpleResponse("value").ToContentByte())
	if !bytes.Equal(res.ToContentByte(), []byte(want)) {
		t.Errorf("ExecGet = %s, want %s", string(res.ToContentByte()), want)
	}
}

func TestExecIncrBy(t *testing.T) {
	db := redis.NewDBInstance(0)

	args := [][]byte{
		[]byte("key"),
		[]byte("1"),
	}
	ExecSet(db, args)

	ExecIncr(db, [][]byte{[]byte("key")})

	v, _ := db.Dataset.Get("key")
	got, _ := v.(string)
	if got != "2" {
		t.Errorf("execIncr should incr key to 2, but key = %s now", got)
	}
}

func TestExecGetset(t *testing.T) {
	db := &redis.RedisDB{
		Dataset: dict.NewDict(6),
		Index:   0,
		TtlMap:  dict.NewDict(6),
	}
	key := "key"
	value := "value"
	db.Dataset.Put(key, value)

	newValue := "newvalue"
	res := ExecGetset(db, [][]byte{
		[]byte("key"),
		[]byte(newValue),
	})
	want := resp.MakeSimpleResponse(value)
	if string(string(want.ToContentByte())) != string(res.ToContentByte()) {
		t.Errorf("execgetSet = %s, want = %s", string(res.ToContentByte()), "+value")
	}
	s := getAsString(db, []byte(key))
	if newValue != s {
		t.Errorf("execgetset store %s , but get %s", "newvalue", s)
	}
}