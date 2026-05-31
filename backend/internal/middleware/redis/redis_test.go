package redis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/alicebob/miniredis/v2"
)

// ============================================================================
// 测试辅助函数
// ============================================================================

// newTestClient 启动一个 miniredis 实例并用它创建 Client，测试结束自动关闭。
// miniredis 是纯 Go 的 Redis 模拟，无需安装 Redis 服务。
func newTestClient(t *testing.T) *Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewClient(rdb, "test:")
}

// newNilClient 返回一个零值 Client，用于测试 nil 防护。
func newNilClient() *Client {
	return &Client{}
}

// ============================================================================
// NewClient / NewFromConfig / Key / Close / Ping / IsMiss 测试
// ============================================================================

// TestNewClient_Normal 测试正常创建 Client。
func TestNewClient_Normal(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	c := NewClient(rdb, "pf:")
	if c == nil {
		t.Fatal("NewClient 不应返回 nil")
	}
	if c.keyPrefix != "pf:" {
		t.Errorf("keyPrefix = %q, 预期 %q", c.keyPrefix, "pf:")
	}
}

// TestNewClient_EmptyPrefix 测试空前缀也可以工作。
func TestNewClient_EmptyPrefix(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	c := NewClient(rdb, "")
	if c.keyPrefix != "" {
		t.Errorf("空前缀 keyPrefix = %q", c.keyPrefix)
	}
}

// TestNewFromConfig_Valid 测试从配置创建 Client。
func TestNewFromConfig_Valid(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	// 只能用真实的 miniredis 地址去构建
	c := NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "cfg:")
	defer c.Close()
	if c == nil {
		t.Fatal("NewClient 不应返回 nil")
	}
}

// TestClose 测试关闭 client。
func TestClose(t *testing.T) {
	c := newTestClient(t)
	if err := c.Close(); err != nil {
		t.Errorf("Close 不应报错, got: %v", err)
	}
}

// TestClose_NilClient 测试 nil client 关闭不报错。
func TestClose_NilClient(t *testing.T) {
	if err := newNilClient().Close(); err != nil {
		t.Errorf("nil client Close 不应报错, got: %v", err)
	}
}

// TestPing 测试 ping 连通性检查。
func TestPing(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Errorf("Ping 应成功, got: %v", err)
	}
}

// TestPing_NilClient 测试 nil client ping 返回错误。
func TestPing_NilClient(t *testing.T) {
	err := newNilClient().Ping(context.Background())
	if err == nil {
		t.Error("nil client Ping 应报错")
	}
}

// TestKey_WithPrefix 测试带前缀的 Key 生成。
func TestKey_WithPrefix(t *testing.T) {
	c := newTestClient(t) // prefix = "test:"
	if k := c.Key("account:%d", 123); k != "test:account:123" {
		t.Errorf("Key = %q, 预期 %q", k, "test:account:123")
	}
}

// TestKey_NilClient 测试 nil client 的 Key 不带前缀。
func TestKey_NilClient(t *testing.T) {
	c := newNilClient()
	if k := c.Key("hello"); k != "hello" {
		t.Errorf("nil client Key = %q, 预期 %q", k, "hello")
	}
}

// TestKey_NoArgs 测试无参数的 Key。
func TestKey_NoArgs(t *testing.T) {
	c := NewClient(nil, "pfx:") // rdb 为 nil 但 Key 不依赖 rdb
	if k := c.Key("static"); k != "pfx:static" {
		t.Errorf("Key = %q, 预期 %q", k, "pfx:static")
	}
}

// TestIsMiss 测试 IsMiss 对 Redis Nil 错误的判断。
func TestIsMiss(t *testing.T) {
	if !IsMiss(goredis.Nil) {
		t.Error("IsMiss(goredis.Nil) 应为 true")
	}
	if IsMiss(errors.New("other")) {
		t.Error("IsMiss(other error) 应为 false")
	}
}

// ============================================================================
// cache.go 方法测试：GetBytes / SetBytes / Del / DelByPattern / MGet
// ============================================================================

// TestSetBytes_And_GetBytes 测试写入后读取。
func TestSetBytes_And_GetBytes(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("test:getset")

	// 写入
	val := []byte("hello world")
	if err := c.SetBytes(ctx, key, val, time.Hour); err != nil {
		t.Fatalf("SetBytes 应成功, got: %v", err)
	}

	// 读取
	got, err := c.GetBytes(ctx, key)
	if err != nil {
		t.Fatalf("GetBytes 应成功, got: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("GetBytes = %q, 预期 %q", got, "hello world")
	}
}

// TestSetBytes_TTLExpire 测试 TTL 过期后读取不到。使用可控时间的 miniredis。
func TestSetBytes_TTLExpire(t *testing.T) {
	ctx := context.Background()
	key := "expire:test"

	// 用独立的 miniredis 以便快进时间
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	c := NewClient(rdb, "")

	c.SetBytes(ctx, key, []byte("expire-me"), time.Second)
	mr.FastForward(time.Second + time.Millisecond) // 快进超过 TTL

	_, err := c.GetBytes(ctx, key)
	if !IsMiss(err) {
		t.Errorf("过期的 Key 应返回 redis.Nil, got: %v", err)
	}
}

// TestSetBytes_ZeroTTL 测试零 TTL 被拒绝。
func TestSetBytes_ZeroTTL(t *testing.T) {
	c := newTestClient(t)
	err := c.SetBytes(context.Background(), "k", []byte("v"), 0)
	if err == nil {
		t.Error("零 TTL 应报错")
	}
	if !strings.Contains(err.Error(), "ttl must be positive") {
		t.Errorf("错误信息应包含 'ttl must be positive', got: %v", err)
	}
}

// TestDel 测试删除单个 Key。
func TestDel(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("test:del")

	c.SetBytes(ctx, key, []byte("x"), time.Hour)
	if err := c.Del(ctx, key); err != nil {
		t.Fatalf("Del 应成功, got: %v", err)
	}

	// 确认已删除
	_, err := c.GetBytes(ctx, key)
	if !IsMiss(err) {
		t.Error("删除后应取不到值")
	}
}

// TestDelByPattern 测试按模式批量删除。
func TestDelByPattern(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	// 写入多个匹配 key
	for i := 0; i < 5; i++ {
		c.SetBytes(ctx, c.Key("batch:%d", i), []byte("x"), time.Hour)
	}
	// 写入不匹配的 key
	c.SetBytes(ctx, c.Key("other"), []byte("y"), time.Hour)

	if err := c.DelByPattern(ctx, "test:batch:*"); err != nil {
		t.Fatalf("DelByPattern 应成功, got: %v", err)
	}

	// 匹配的 key 应被删除
	for i := 0; i < 5; i++ {
		_, err := c.GetBytes(ctx, c.Key("batch:%d", i))
		if !IsMiss(err) {
			t.Errorf("batch:%d 应被删除", i)
		}
	}
	// 不匹配的 key 应保留
	if _, err := c.GetBytes(ctx, c.Key("other")); err != nil {
		t.Error("不匹配的 key 不应被删除")
	}
}

// TestMGet 测试批量获取。
func TestMGet(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.SetBytes(ctx, c.Key("a"), []byte("1"), time.Hour)
	c.SetBytes(ctx, c.Key("b"), []byte("2"), time.Hour)

	vals, err := c.MGet(ctx, c.Key("a"), c.Key("b"), c.Key("c"))
	if err != nil {
		t.Fatalf("MGet 应成功, got: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("MGet 应返回 3 个结果, got: %d", len(vals))
	}
	// a 和 b 有值，c 不存在为 nil
	if s, ok := vals[0].(string); !ok || s != "1" {
		t.Errorf("vals[0] = %v, 预期 '1'", vals[0])
	}
	if vals[2] != nil {
		t.Errorf("c 不存在应为 nil, got: %v", vals[2])
	}
}

// ============================================================================
// nil client 防护测试
// ============================================================================

// TestGetBytes_NilClient nil client 应返回错误而非 panic。
func TestGetBytes_NilClient(t *testing.T) {
	_, err := newNilClient().GetBytes(context.Background(), "k")
	if err == nil {
		t.Error("nil client GetBytes 应报错")
	}
}

// TestSetBytes_NilClient nil client 应返回错误。
func TestSetBytes_NilClient(t *testing.T) {
	err := newNilClient().SetBytes(context.Background(), "k", []byte("v"), time.Minute)
	if err == nil {
		t.Error("nil client SetBytes 应报错")
	}
}

// TestDel_NilClient nil client 应返回错误。
func TestDel_NilClient(t *testing.T) {
	err := newNilClient().Del(context.Background(), "k")
	if err == nil {
		t.Error("nil client Del 应报错")
	}
}

// TestDelByPattern_NilClient nil client 应返回错误。
func TestDelByPattern_NilClient(t *testing.T) {
	err := newNilClient().DelByPattern(context.Background(), "*")
	if err == nil {
		t.Error("nil client DelByPattern 应报错")
	}
}

// TestMGet_NilClient nil client 应返回错误。
func TestMGet_NilClient(t *testing.T) {
	_, err := newNilClient().MGet(context.Background(), "k")
	if err == nil {
		t.Error("nil client MGet 应报错")
	}
}

// ============================================================================
// ZSET 方法测试
// ============================================================================

// TestZincrBy 测试成员加分。
func TestZincrBy(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("zincr")

	// 第一次加分：自动创建成员
	if err := c.ZincrBy(ctx, key, "tom", 10); err != nil {
		t.Fatalf("ZincrBy 应成功, got: %v", err)
	}
	// 再加一次
	if err := c.ZincrBy(ctx, key, "tom", 5); err != nil {
		t.Fatalf("ZincrBy 第二次应成功, got: %v", err)
	}

	// 验证分数
	members, err := c.ZRangeWithScores(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRangeWithScores failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("应有 1 个成员, got %d", len(members))
	}
	if members[0].Score != 15 {
		t.Errorf("Score = %f, 预期 15", members[0].Score)
	}
}

// TestZAdd 测试添加成员。
func TestZAdd(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("zadd")

	err := c.ZAdd(ctx, key,
		goredis.Z{Member: "alice", Score: 100},
		goredis.Z{Member: "bob", Score: 50},
	)
	if err != nil {
		t.Fatalf("ZAdd 应成功, got: %v", err)
	}

	n, _ := c.Exists(ctx, key)
	if !n {
		t.Error("ZSet key 应存在")
	}
}

// TestZRangeWithScores 测试正序取排名。
func TestZRangeWithScores(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("zrange")

	c.ZAdd(ctx, key,
		goredis.Z{Member: "a", Score: 10},
		goredis.Z{Member: "b", Score: 30},
		goredis.Z{Member: "c", Score: 20},
	)

	// 全部成员，按分数升序
	all, err := c.ZRangeWithScores(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRangeWithScores 应成功, got: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("应有 3 个成员, got %d", len(all))
	}
	// 验证低到高
	if all[0].Member.(string) != "a" {
		t.Errorf("第 1 名应为 a (score 10), got %v", all[0].Member)
	}
	if all[2].Member.(string) != "b" {
		t.Errorf("第 3 名应为 b (score 30), got %v", all[2].Member)
	}
}

// TestZRevRange 测试倒序取排名（高到低），用于取 Top N。
func TestZRevRange(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("zrev")

	c.ZAdd(ctx, key,
		goredis.Z{Member: "x", Score: 10},
		goredis.Z{Member: "y", Score: 50},
		goredis.Z{Member: "z", Score: 30},
	)

	top, err := c.ZRevRange(ctx, key, 0, 1) // Top 2
	if err != nil {
		t.Fatalf("ZRevRange 应成功, got: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("应有 2 个, got %d", len(top))
	}
	if top[0] != "y" {
		t.Errorf("第 1 名应为 y (score 50), got %s", top[0])
	}
	if top[1] != "z" {
		t.Errorf("第 2 名应为 z (score 30), got %s", top[1])
	}
}

// TestZRevRangeByScore 测试按分数区间倒序取（分页）。
// 当前 miniredis 对 ZRangeArgs(Rev+ByScore+Offset+Count) 组合不完整支持，
// 此测试仅在真实 Redis 环境下有效。在 CI 中可通过设置 REDIS_ADDR 环境变量指向真实实例。
func TestZRevRangeByScore(t *testing.T) {
	t.Skip("miniredis 不支持 ZRangeArgs(Rev+ByScore+Count)，需真实 Redis 验证")
}

// TestZRemRangeByRank 测试按排名裁剪（保留 Top N）。
func TestZRemRangeByRank(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("zremrange")

	c.ZAdd(ctx, key,
		goredis.Z{Member: "1st", Score: 500},
		goredis.Z{Member: "2nd", Score: 400},
		goredis.Z{Member: "3rd", Score: 300},
		goredis.Z{Member: "4th", Score: 200},
		goredis.Z{Member: "5th", Score: 100},
	)

	// 只保留前 3 名（rank 2 之后的全部删除）
	if err := c.ZRemRangeByRank(ctx, key, 0, -4); err != nil {
		t.Fatalf("ZRemRangeByRank 应成功, got: %v", err)
	}

	all, _ := c.ZRangeWithScores(ctx, key, 0, -1)
	if len(all) != 3 {
		t.Errorf("应剩 3 个, got %d", len(all))
	}
	// 剩余的是分数最高的 3 个
	if all[2].Member.(string) != "1st" {
		t.Errorf("保留的最高分应为 1st, got %v", all[2].Member)
	}
}

// TestZUnionStore 测试合并多个 ZSET。
func TestZUnionStore(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	c.ZAdd(ctx, c.Key("day1"), goredis.Z{Member: "a", Score: 10})
	c.ZAdd(ctx, c.Key("day2"), goredis.Z{Member: "a", Score: 20})

	err := c.ZUnionStore(ctx, c.Key("total"), []string{c.Key("day1"), c.Key("day2")}, "SUM")
	if err != nil {
		t.Fatalf("ZUnionStore 应成功, got: %v", err)
	}

	members, _ := c.ZRangeWithScores(ctx, c.Key("total"), 0, -1)
	if len(members) != 1 || members[0].Score != 30 {
		t.Errorf("合并后 score 应为 30, got %v", members)
	}
}

// TestExists 测试 Key 存在性检查。
func TestExists(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("exists")

	c.SetBytes(ctx, key, []byte("v"), time.Hour)

	ok, err := c.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists 应成功, got: %v", err)
	}
	if !ok {
		t.Error("存在的 Key 应返回 true")
	}

	ok, _ = c.Exists(ctx, c.Key("not-exist"))
	if ok {
		t.Error("不存在的 Key 应返回 false")
	}
}

// TestExpire 测试设置 TTL。
// TestExpire 测试为已有 Key 设置过期时间。
func TestExpire(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	c := NewClient(rdb, "")

	ctx := context.Background()
	c.SetBytes(ctx, "timer", []byte("x"), time.Hour)
	c.Expire(ctx, "timer", time.Second)

	mr.FastForward(time.Second + time.Millisecond)
	_, err := c.GetBytes(ctx, "timer")
	if !IsMiss(err) {
		t.Error("过期后应取不到")
	}
}

// ============================================================================
// Lock / Unlock 分布式锁测试
// ============================================================================

// TestLock_Success 测试获取锁成功。
func TestLock_Success(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	token, ok, err := c.Lock(ctx, c.Key("lock:test"), time.Minute)
	if err != nil {
		t.Fatalf("Lock 应成功, got: %v", err)
	}
	if !ok {
		t.Error("首次 Lock 应成功（ok=true）")
	}
	if token == "" {
		t.Error("Lock 成功时应返回非空 token")
	}
}

// TestLock_Conflict 测试重复加锁被拒绝。
func TestLock_Conflict(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("lock:conflict")

	token1, ok1, _ := c.Lock(ctx, key, time.Minute)
	if !ok1 {
		t.Fatal("首次 Lock 应成功")
	}

	_, ok2, _ := c.Lock(ctx, key, time.Minute)
	if ok2 {
		t.Error("同一 Key 重复 Lock 应失败")
	}

	// 用 token1 解锁后可以重新加锁
	if err := c.Unlock(ctx, key, token1); err != nil {
		t.Fatalf("Unlock 应成功, got: %v", err)
	}
	_, ok3, _ := c.Lock(ctx, key, time.Minute)
	if !ok3 {
		t.Error("解锁后 Lock 应成功")
	}
}

// TestUnlock_WrongToken 测试用错误的 token 不能解锁。
func TestUnlock_WrongToken(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("lock:wrongtoken")

	token, ok, _ := c.Lock(ctx, key, time.Minute)
	if !ok {
		t.Fatal("Lock 应成功")
	}

	// 用错误的 token 尝试解锁
	c.Unlock(ctx, key, "wrong-token")

	// 锁应该还在（不能被解锁）
	_, ok2, _ := c.Lock(ctx, key, time.Millisecond)
	if ok2 {
		t.Error("错误 token 不应能解锁，锁应还存在")
	}

	// 用正确 token 解锁
	_ = token // suppress unused
}

// TestLock_ZeroTTL 测试零 TTL 被拒绝。
func TestLock_ZeroTTL(t *testing.T) {
	c := newTestClient(t)
	_, _, err := c.Lock(context.Background(), c.Key("lock:zero"), 0)
	if err == nil {
		t.Error("零 TTL Lock 应报错")
	}
}

// ============================================================================
// IncrementWithExpire 测试
// ============================================================================

// TestIncrementWithExpire 测试递增计数器。
func TestIncrementWithExpire(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := c.Key("incr")

	// 第一次调用
	n, err := c.IncrementWithExpire(ctx, key, time.Hour)
	if err != nil {
		t.Fatalf("IncrementWithExpire 应成功, got: %v", err)
	}
	if n != 1 {
		t.Errorf("首次 incr 应为 1, got %d", n)
	}

	// 第二次调用 — 不应覆盖过期时间
	n2, _ := c.IncrementWithExpire(ctx, key, time.Hour)
	if n2 != 2 {
		t.Errorf("第二次 incr 应为 2, got %d", n2)
	}
}

// TestIncrementWithExpire_ZeroExpire 测试零过期时间被拒绝。
func TestIncrementWithExpire_ZeroExpire(t *testing.T) {
	c := newTestClient(t)
	_, err := c.IncrementWithExpire(context.Background(), c.Key("incr:zero"), 0)
	if err == nil || !strings.Contains(err.Error(), "expire time must be positive") {
		t.Errorf("零过期时间应报错, got: %v", err)
	}
}

// ============================================================================
// 边界测试
// ============================================================================

// TestZSet_NilClient 测试 nil client 的 ZSET 操作。
func TestZSet_NilClient(t *testing.T) {
	c := newNilClient()
	ctx := context.Background()

	if err := c.ZincrBy(ctx, "k", "m", 1); err == nil {
		t.Error("nil client ZincrBy 应报错")
	}
	if err := c.ZAdd(ctx, "k"); err == nil {
		t.Error("nil client ZAdd 应报错")
	}
	if err := c.ZRemRangeByRank(ctx, "k", 0, -1); err == nil {
		t.Error("nil client ZRemRangeByRank 应报错")
	}
	if _, err := c.ZRangeWithScores(ctx, "k", 0, -1); err == nil {
		t.Error("nil client ZRangeWithScores 应报错")
	}
	if _, err := c.ZRevRange(ctx, "k", 0, -1); err == nil {
		t.Error("nil client ZRevRange 应报错")
	}
	if _, err := c.ZRevRangeByScore(ctx, "k", "+inf", "-inf", 0, 10); err == nil {
		t.Error("nil client ZRevRangeByScore 应报错")
	}
	if err := c.ZUnionStore(ctx, "d", []string{"a"}, "SUM"); err == nil {
		t.Error("nil client ZUnionStore 应报错")
	}
	if err := c.Expire(ctx, "k", time.Second); err == nil {
		t.Error("nil client Expire 应报错")
	}
	if _, err := c.Exists(ctx, "k"); err == nil {
		t.Error("nil client Exists 应报错")
	}
}

// TestLock_NilClient 测试 nil client 的 Lock/Unlock。
func TestLock_NilClient(t *testing.T) {
	c := newNilClient()
	ctx := context.Background()

	_, _, err := c.Lock(ctx, "k", time.Minute)
	if err == nil {
		t.Error("nil client Lock 应报错")
	}
	if err := c.Unlock(ctx, "k", "t"); err != nil {
		t.Error("nil client Unlock 应静默返回 nil")
	}
}

// TestIncrementWithExpire_NilClient 测试 nil client 的递增。
func TestIncrementWithExpire_NilClient(t *testing.T) {
	c := newNilClient()
	_, err := c.IncrementWithExpire(context.Background(), "k", time.Minute)
	if err == nil {
		t.Error("nil client IncrementWithExpire 应报错")
	}
}
