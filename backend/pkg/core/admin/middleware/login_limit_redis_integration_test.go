package middleware

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	commonRedis "joincare_server/pkg/common/database/redis"
	coreDB "joincare_server/pkg/core/db"
)

func openLoginLimitRedis(t *testing.T) *commonRedis.Connection {
	t.Helper()
	address := os.Getenv("JOINCARE_REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("JOINCARE_REDIS_TEST_ADDR is not set")
	}
	connection, err := commonRedis.Connect([]string{address}, os.Getenv("JOINCARE_REDIS_TEST_PASSWORD"))
	if err != nil {
		t.Fatal(err)
	}
	previous := coreDB.Client.Redis
	coreDB.Client.Redis = connection
	t.Cleanup(func() { coreDB.Client.Redis = previous; connection.Close() })
	return connection
}

func TestAdminLoginLimitAgainstRedisCluster(t *testing.T) {
	connection := openLoginLimitRedis(t)
	ctx := context.Background()
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	ip := "198.51.100." + stamp[len(stamp)-3:]
	username := "security-" + stamp + "@joincare.test"
	userHash := hashedLoginIdentity(username)
	keys := []string{"ADMLOGIN:IP:" + ip, "ADMLOGIN:U:" + userHash, "ADMLOGIN:IU:" + ip + ":" + userHash, "ADMLOGIN:ALERT:" + userHash}
	t.Cleanup(func() {
		for _, key := range keys {
			connection.Client.Del(ctx, key)
		}
	})

	for attempt := 1; attempt <= 5; attempt++ {
		if err := CheckAdminLoginLimit(ctx, ip, username); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	if err := CheckAdminLoginLimit(ctx, ip, username); !errors.Is(err, ErrLoginRateLimited) {
		t.Fatalf("sixth combined attempt was not limited: %v", err)
	}
	if ttl := connection.Client.PTTL(ctx, "ADMLOGIN:IU:"+ip+":"+userHash).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected fixed-window TTL: %v", ttl)
	}
	if !ShouldSendLoginAlert(ctx, username) {
		t.Fatal("first alert was not admitted")
	}
	if ShouldSendLoginAlert(ctx, username) {
		t.Fatal("duplicate alert was not suppressed")
	}
}

func TestFixedWindowIncrementIsAtomicAgainstRedisCluster(t *testing.T) {
	connection := openLoginLimitRedis(t)
	ctx := context.Background()
	key := "{joincare-security-test}:fixed-window:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { connection.Client.Del(ctx, key) })
	const workers = 50
	results := make(chan int64, workers)
	errorsCh := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, err := connection.FixedWindowIncrement(ctx, key, time.Minute)
			results <- value
			errorsCh <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[int64]bool, workers)
	for value := range results {
		seen[value] = true
	}
	if len(seen) != workers || !seen[1] || !seen[workers] {
		t.Fatalf("atomic increments missing: %v", seen)
	}
	if ttl := connection.Client.PTTL(ctx, key).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected TTL: %v", ttl)
	}
}
