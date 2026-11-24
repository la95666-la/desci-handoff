package redis

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"joincare_server/pkg/common/log"
)

type Connection struct {
	Client *redis.ClusterClient
}

func Connect(addr []string, password string) (*Connection, error) {
	var err error
	c := &Connection{}
	c.Client, err = initRedis(addr, password)
	return c, err
}

type redisHook struct{}

func (redisHook) DialHook(hook redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		log.GetLogger().Infof("dialing %s %s\n", network, addr)
		conn, err := hook(ctx, network, addr)
		log.GetLogger().Infof("finished dialing %s %s\n", network, addr)
		return conn, err
	}
}

func (redisHook) ProcessHook(hook redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := hook(ctx, cmd)
		log.GetLogger().Infof("finished processing: <%s>\n", cmd)
		return err
	}
}

func (redisHook) ProcessPipelineHook(hook redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		log.GetLogger().Infof("pipeline starting processing: %v\n", cmds)
		err := hook(ctx, cmds)
		log.GetLogger().Infof("pipeline finished processing: %v\n", cmds)
		return err
	}
}

func initRedis(address []string, password string) (*redis.ClusterClient, error) {
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           address,
		Password:        password,
		MaxRetries:      5,
		DialTimeout:     10 * time.Second,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		PoolSize:        4 * runtime.GOMAXPROCS(0),
		MaxIdleConns:    4 * runtime.GOMAXPROCS(0),
		ConnMaxIdleTime: 10 * time.Second,
		ConnMaxLifetime: 10 * time.Second,
	})
	//if config.Test() {
	//	rdb.AddHook(redisHook{})
	//}
	_, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("Error connecting to Redis cluster:", err)
		return nil, err
	}

	return rdb, nil
}

func (c Connection) Keys(ctx context.Context, pattern string) []string {
	cmd := c.Client.Keys(ctx, pattern)
	if cmd.Err() != nil {
		return []string{}
	}
	return cmd.Val()
}

func (c Connection) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	err := c.Client.Set(ctx, key, value, expiration).Err()
	return err
}

func (c Connection) SetKeepTTL(ctx context.Context, key string, value interface{}) error {
	err := c.Client.Set(ctx, key, value, redis.KeepTTL).Err()
	return err
}

func (c Connection) SetNoExp(ctx context.Context, key string, value interface{}) error {
	err := c.Client.Set(ctx, key, value, 0).Err()
	return err
}

func (c Connection) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) bool {
	cmd := c.Client.SetNX(ctx, key, value, expiration)
	return cmd.Val()
}

// TryTokenLock 使用调用方唯一 token 获取带 TTL 的分布式锁。
func (c Connection) TryTokenLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	return c.Client.SetNX(ctx, key, token, ttl).Result()
}

// RefreshTokenLock 仅锁持有者可续租。
func (c Connection) RefreshTokenLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	script := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`
	result, err := c.Client.Eval(ctx, script, []string{key}, token, ttl.Milliseconds()).Int64()
	return result == 1, err
}

// UnlockTokenLock 仅锁持有者可释放，避免过期后误删其他实例的新锁。
func (c Connection) UnlockTokenLock(ctx context.Context, key, token string) error {
	script := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
	return c.Client.Eval(ctx, script, []string{key}, token).Err()
}

// DeleteIfValue atomically consumes a one-time value only when it still equals
// the value previously verified by the caller.
func (c Connection) DeleteIfValue(ctx context.Context, key, expected string) (bool, error) {
	script := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
	result, err := c.Client.Eval(ctx, script, []string{key}, expected).Int64()
	return result == 1, err
}

// ReserveCappedIncrement atomically reserves delta while keeping the resulting
// counter at or below cap. It returns the new value, or -1 when the cap would be
// exceeded.
func (c Connection) ReserveCappedIncrement(ctx context.Context, key string, delta, cap int64, ttl time.Duration) (int64, error) {
	script := `local current=tonumber(redis.call("get",KEYS[1]) or "0"); local delta=tonumber(ARGV[1]); local cap=tonumber(ARGV[2]); if current+delta>cap then return -1 end; local next=current+delta; redis.call("set",KEYS[1],next,"PX",ARGV[3]); return next`
	return c.Client.Eval(ctx, script, []string{key}, delta, cap, ttl.Milliseconds()).Int64()
}

func (c Connection) ReleaseCappedIncrement(ctx context.Context, key string, delta int64) error {
	script := `local current=tonumber(redis.call("get",KEYS[1]) or "0"); local next=current-tonumber(ARGV[1]); if next<=0 then return redis.call("del",KEYS[1]) end; redis.call("set",KEYS[1],next,"KEEPTTL"); return next`
	return c.Client.Eval(ctx, script, []string{key}, delta).Err()
}

// SetIfTokenLock 仅当前锁持有者可写缓存值，用于 checkpoint fencing。
func (c Connection) SetIfTokenLock(ctx context.Context, lockKey, token, valueKey string, value interface{}) (bool, error) {
	script := `if redis.call("get", KEYS[1]) == ARGV[1] then redis.call("set", KEYS[2], ARGV[2]); return 1 else return 0 end`
	result, err := c.Client.Eval(ctx, script, []string{lockKey, valueKey}, token, value).Int64()
	return result == 1, err
}

func (c Connection) Get(ctx context.Context, key string) string {
	cmd := c.Client.Get(ctx, key)
	err := cmd.Err()
	if err != nil {
		return ""
	}
	return cmd.Val()
}

func (c Connection) GetOptional(ctx context.Context, key string) (string, bool, error) {
	value, err := c.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (c Connection) GetAsInt(ctx context.Context, key string) int {
	cmd := c.Client.Get(ctx, key)
	err := cmd.Err()
	if err != nil {
		return 0
	}
	v := cmd.Val()
	number, err := strconv.Atoi(v)
	if err != nil {

		return 0
	}
	return number
}

func (c Connection) GetAsBool(ctx context.Context, key string) (bool, error) {
	cmd := c.Client.Get(ctx, key)
	err := cmd.Err()
	if err != nil {
		return false, err
	}
	v := cmd.Val()
	res, err := strconv.ParseBool(v)
	if err != nil {

		return false, err
	}
	return res, nil
}

func (c Connection) GetByJson(ctx context.Context, key string, v any) error {
	cmd := c.Client.Get(ctx, key)
	if cmd.Val() == "" {
		return errors.New("key not found")
	}
	b, err := cmd.Bytes()
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, v)
	return err
}

func (c Connection) MGet(ctx context.Context, key ...string) map[string]string {
	res := make(map[string]string, len(key))
	if len(key) == 0 {
		return res
	}

	// Redis Cluster requires every key in a single MGET command to share a
	// hash slot. Queue independent GET commands instead; ClusterClient splits
	// the pipeline by node/slot while preserving the input order.
	commands := make([]*redis.StringCmd, len(key))
	_, _ = c.Client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, currentKey := range key {
			commands[i] = pipe.Get(ctx, currentKey)
		}
		return nil
	})

	for i, command := range commands {
		if command == nil || command.Err() != nil {
			continue
		}
		res[key[i]] = command.Val()
	}
	return res
}

func (c Connection) MSCard(ctx context.Context, key ...string) map[string]int64 {
	res := make(map[string]int64, len(key))
	if len(key) == 0 {
		return res
	}

	// Use independent SCARD commands so keys may live in different cluster
	// hash slots. ClusterClient groups the pipeline by node automatically.
	commands := make([]*redis.IntCmd, len(key))
	_, _ = c.Client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, currentKey := range key {
			commands[i] = pipe.SCard(ctx, currentKey)
		}
		return nil
	})

	for i, command := range commands {
		if command == nil || command.Err() != nil {
			continue
		}
		res[key[i]] = command.Val()
	}
	return res
}

func (c Connection) SetByJson(ctx context.Context, key string, value any, expire time.Duration) error {

	bs, err := json.Marshal(value)
	if err != nil {

		return err
	}
	if expire < time.Second {
		expire *= time.Second
	}
	return c.Set(ctx, key, string(bs), expire)
}

func (c Connection) SetByJsonNoExpire(ctx context.Context, key string, value any) error {

	bs, err := json.Marshal(value)
	if err != nil {

		return err
	}
	return c.Set(ctx, key, string(bs), 0)
}

func (c Connection) SetByJsonXX(ctx context.Context, key string, value any, expire time.Duration) bool {
	bs, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return c.Client.SetXX(ctx, key, string(bs), expire).Val()
}

func (c Connection) Exists(ctx context.Context, key string) bool {
	cmd := c.Client.Exists(ctx, key)

	return cmd.Val() == 1
}

func (c Connection) Expire(ctx context.Context, key string, ttl time.Duration) bool {
	cmd := c.Client.Expire(ctx, key, ttl)
	return cmd.Val()
}

func (c Connection) TTL(ctx context.Context, key string) int {
	cmd := c.Client.TTL(ctx, key)
	second := cmd.Val() / time.Second
	return int(second)
}

func (c Connection) Del(ctx context.Context, key ...string) error {
	err := c.Client.Del(ctx, key...).Err()
	return err
}

// FixedWindowIncrement atomically increments a fixed-window counter and sets
// its TTL on first use. Callers that protect privileged authentication can
// fail closed when this returns an error.
func (c Connection) FixedWindowIncrement(ctx context.Context, key string, window time.Duration) (int64, error) {
	script := `local n=redis.call("incr",KEYS[1]); if n==1 then redis.call("pexpire",KEYS[1],ARGV[1]) end; return n`
	return c.Client.Eval(ctx, script, []string{key}, window.Milliseconds()).Int64()
}

func (c Connection) Incr(ctx context.Context, key string) int64 {
	cmd := c.Client.Incr(ctx, key)
	err := cmd.Err()
	if err != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) IncrBy(ctx context.Context, key string, v int64) int64 {
	cmd := c.Client.IncrBy(ctx, key, v)
	err := cmd.Err()
	if err != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) IncrByFloat(ctx context.Context, key string, v float64) float64 {
	cmd := c.Client.IncrByFloat(ctx, key, v)
	err := cmd.Err()
	if err != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) Decr(ctx context.Context, key string) int64 {
	cmd := c.Client.Decr(ctx, key)
	err := cmd.Err()
	if err != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) DecrBy(ctx context.Context, key string, v int) int64 {
	cmd := c.Client.DecrBy(ctx, key, int64(v))
	err := cmd.Err()
	if err != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) SetBit(ctx context.Context, key string, offset int64, value int) int {
	cmd := c.Client.SetBit(ctx, key, offset, value)
	return int(cmd.Val())
}

func (c Connection) LLen(ctx context.Context, key string) int64 {
	cmd := c.Client.LLen(ctx, key)
	err := cmd.Err()
	if err != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) LPop(ctx context.Context, key string) string {
	cmd := c.Client.LPop(ctx, key)
	err := cmd.Err()
	if err != nil {
		return ""
	}
	return cmd.Val()
}

func (c Connection) LPopCount(ctx context.Context, key string, count int) []string {
	cmd := c.Client.LPopCount(ctx, key, count)
	err := cmd.Err()
	if err != nil {
		return nil
	}
	return cmd.Val()
}

func (c Connection) LPos(ctx context.Context, key string, value string) int64 {
	cmd := c.Client.LPos(ctx, key, value, redis.LPosArgs{
		Rank:   0,
		MaxLen: -1,
	})
	return cmd.Val()
}

func (c Connection) LPush(ctx context.Context, key string, values ...interface{}) int64 {
	cmd := c.Client.LPush(ctx, key, values...)
	return cmd.Val()
}

func (c Connection) LRange(ctx context.Context, key string, start, stop int64) []string {
	cmd := c.Client.LRange(ctx, key, start, stop)
	return cmd.Val()
}

func (c Connection) LRem(ctx context.Context, key string, count int64, value interface{}) int64 {
	cmd := c.Client.LRem(ctx, key, count, value)
	return cmd.Val()
}

func (c Connection) LSet(ctx context.Context, key string, index int64, value interface{}) error {
	cmd := c.Client.LSet(ctx, key, index, value)
	return cmd.Err()
}

func (c Connection) LTrim(ctx context.Context, key string, start, stop int64) error {
	cmd := c.Client.LTrim(ctx, key, start, stop)
	return cmd.Err()
}

func (c Connection) RPush(ctx context.Context, key string, value ...any) error {
	cmd := c.Client.RPush(ctx, key, value...)
	return cmd.Err()
}

type Z struct {
	Score  float64
	Member any
}

func (c Connection) ZAdd(ctx context.Context, key string, members ...Z) error {
	ZSet := make([]redis.Z, 0, len(members))
	for _, value := range members {
		ZSet = append(ZSet, redis.Z{
			Score:  value.Score,
			Member: value.Member,
		})
	}
	return c.Client.ZAdd(ctx, key, ZSet...).Err()
}

func (c Connection) ZCard(ctx context.Context, key string) (int64, error) {

	cmd := c.Client.ZCard(ctx, key)
	err := cmd.Err()
	if err != nil {
		return 0, err
	}
	return cmd.Val(), nil
}

func (c Connection) ZScore(ctx context.Context, key, member string) (float64, error) {

	cmd := c.Client.ZScore(ctx, key, member)
	err := cmd.Err()
	if err != nil {

		return 0, err
	}
	return cmd.Val(), nil
}

func (c Connection) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {

	cmd := c.Client.ZRevRange(ctx, key, start, stop)
	err := cmd.Err()
	if err != nil {

		return nil, err
	}
	return cmd.Val(), nil
}

func (c Connection) ZRevRangeWithScore(ctx context.Context, key string, start, stop int64) ([]Z, error) {
	cmd := c.Client.ZRevRangeWithScores(ctx, key, start, stop)
	err := cmd.Err()
	if err != nil {
		return nil, err
	}
	res := make([]Z, 0, len(cmd.Val()))
	for _, z := range cmd.Val() {
		res = append(res, Z{
			Score:  z.Score,
			Member: z.Member.(string),
		})
	}
	return res, nil
}

func (c Connection) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {

	cmd := c.Client.ZRange(ctx, key, start, stop)
	err := cmd.Err()
	if err != nil {

		return nil, err
	}
	return cmd.Val(), nil
}

func (c Connection) ZRangeByScoreWithScores(ctx context.Context, key string, start, stop int64, offset, count int64) ([]Z, error) {

	opt := redis.ZRangeBy{
		Min:    strconv.Itoa(int(start)),
		Max:    strconv.Itoa(int(stop)),
		Offset: int64(offset),
		Count:  count,
	}
	cmd := c.Client.ZRangeByScoreWithScores(ctx, key, &opt)
	err := cmd.Err()
	if err != nil {
		return nil, err
	}
	values := make(map[float64]any)
	for _, v := range cmd.Val() {

		values[v.Score] = v.Member
	}
	res := make([]Z, 0, len(cmd.Val()))
	for _, z := range cmd.Val() {
		res = append(res, Z{
			Score:  z.Score,
			Member: z.Member.(string),
		})
	}
	return res, nil
}

func (c Connection) ZRevRangeByScoreWithScores(ctx context.Context, key string, max, min int64, offset, count int64) ([]Z, error) {

	opt := redis.ZRangeBy{
		Min:    strconv.Itoa(int(min)),
		Max:    strconv.Itoa(int(max)),
		Offset: int64(offset),
		Count:  count,
	}
	cmd := c.Client.ZRevRangeByScoreWithScores(ctx, key, &opt)
	err := cmd.Err()
	if err != nil {
		return nil, err
	}
	values := make(map[float64]any)
	for _, v := range cmd.Val() {

		values[v.Score] = v.Member
	}
	res := make([]Z, 0, len(cmd.Val()))
	for _, z := range cmd.Val() {
		res = append(res, Z{
			Score:  z.Score,
			Member: z.Member.(string),
		})
	}
	return res, nil
}

func (c Connection) ZIncrBy(ctx context.Context, key string, increment float64, member string) float64 {

	return c.Client.ZIncrBy(ctx, key, increment, member).Val()
}

func (c Connection) ZRem(ctx context.Context, key string, members ...interface{}) int64 {
	return c.Client.ZRem(ctx, key, members...).Val()
}

func (c Connection) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) int64 {
	cmd := c.Client.ZRemRangeByRank(ctx, key, start, stop)
	if cmd.Err() != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) SAdd(ctx context.Context, key string, member ...string) int64 {
	addMember := make([]interface{}, 0, len(member))
	for _, s := range member {
		addMember = append(addMember, s)
	}
	cmd := c.Client.SAdd(ctx, key, addMember...)
	if cmd.Err() != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) SRem(ctx context.Context, key string, member ...string) int64 {
	remMember := make([]interface{}, 0, len(member))
	for _, s := range member {
		remMember = append(remMember, s)
	}
	cmd := c.Client.SRem(ctx, key, remMember...)
	if cmd.Err() != nil {
		return 0
	}
	return cmd.Val()
}

func (c Connection) SIsMember(ctx context.Context, key string, member string) bool {
	cmd := c.Client.SIsMember(ctx, key, member)
	if cmd.Err() != nil {
		return false
	}
	return cmd.Val()
}

func (c Connection) SScan(ctx context.Context, key string, cursor uint64, match string, count int64) ([]string, uint64, error) {
	cmd := c.Client.SScan(ctx, key, cursor, match, count)
	return cmd.Result()
}

func (c Connection) SCard(ctx context.Context, key string) (int64, error) {

	return c.Client.SCard(ctx, key).Result()
}

func (c Connection) SScanAll(ctx context.Context, key string) []string {
	// 初始化游标
	var cursor uint64 = 0
	// 存储所有的元素
	var allElements []string
	// 迭代获取集合中的所有元素
	for {
		// 使用 SScan 命令获取元素
		result, nextCursor, err := c.SScan(ctx, key, cursor, "", 0)
		if err != nil {
			return []string{}
		}
		// 将获取到的元素添加到切片中
		allElements = append(allElements, result...)
		// 更新游标
		cursor = nextCursor
		// 如果游标为0，表示迭代结束
		if cursor == 0 {
			break
		}
	}
	return allElements
}

func (c Connection) ScanAll(ctx context.Context, key string, count int64) []string {
	// 初始化游标
	var cursor uint64 = 0
	// 存储所有的元素
	var allElements []string
	// 迭代获取集合中的所有元素
	for {
		// 使用 Scan 命令获取元素
		result, nextCursor, err := c.Client.Scan(ctx, cursor, key, count).Result()
		if err != nil {
			return []string{}
		}
		// 将获取到的元素添加到切片中
		allElements = append(allElements, result...)
		// 更新游标
		cursor = nextCursor
		// 如果游标为0，表示迭代结束
		if cursor == 0 {
			break
		}
	}
	return allElements
}

func (c Connection) RedisSync(f func() any, syncKey string) (any, error) {

	ctx := context.Background()
	maxRetries := 5
	retryDelay := time.Millisecond * 300

	defer func() {
		// 释放锁，删除键
		err := c.Del(ctx, syncKey)
		if err != nil {
			log.GetLogger().WithFields(logrus.Fields{"syncKey": syncKey, "err": err}).Error("Error releasing lock")
		}
	}()

	for i := 0; i < maxRetries; i++ {
		// 使用SET key value nx ex 尝试获取锁
		// EX 表示设置键的过期时间，这里假设是 60 秒
		ok := c.SetNX(ctx, syncKey, "1", 60*time.Second)
		if !ok {

			// 锁已被占用，返回特定错误或nil表示无法获取锁
			log.GetLogger().WithFields(logrus.Fields{"syncKey": syncKey}).Info("lock is already acquired by someone else.")
			time.Sleep(retryDelay)
			continue
		}
		// 锁成功
		if f != nil {
			res := f()
			return res, nil
		}
	}

	// 达到最大重试次数后仍未获取锁
	log.GetLogger().WithFields(logrus.Fields{"syncKey": syncKey}).Error(fmt.Sprintf("failed to acquire lock after %d retries", maxRetries))
	if f != nil {
		res := f()
		return res, nil
	}
	return nil, nil
}

// DistributedLock 分布式阻塞锁
func (c Connection) DistributedLock(ctx context.Context, key string, expiration time.Duration) {
	pubsub := c.Client.PSubscribe(ctx, fmt.Sprintf("__event:%s", key))
	defer pubsub.Close()
	// 先加一把锁
	cmd := c.Client.SetNX(ctx, key, 1, expiration)
	if cmd.Val() {
		// 加锁成功 直接返回
		return
	}
	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			log.GetLogger().Errorf("Error receiving message: %s", err)
			break
		}
		// 非过期 删除事件不处理
		if msg.Payload != "unlock" {
			continue
		}
		// 再次尝试设锁
		cmd = c.Client.SetNX(ctx, key, 1, expiration)
		if cmd.Val() {
			// 加锁成功 直接返回
			return
		}
	}
}

// UnDistributedLock 分布式阻塞解锁
func (c Connection) UnDistributedLock(ctx context.Context, key string) {
	c.Client.Del(ctx, key)
	c.Client.Publish(ctx, fmt.Sprintf("__event:%s", key), "unlock")
}

func (c Connection) Close() {
	c.Client.Close()
}
