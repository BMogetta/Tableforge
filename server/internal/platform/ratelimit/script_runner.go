package ratelimit

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// ScriptArgs groups all parameters required by the Redis rate-limit script.
// Using a struct prevents parameter ordering mistakes and improves readability.
type ScriptArgs struct {
	Key    string
	NowNs  int64
	WinNs  int64
	Limit  int
	TTLms  int64
	Member string
}

// ScriptRunner abstracts execution of the Redis Lua script.
// This allows unit tests to replace Redis with a fake implementation.
type ScriptRunner interface {
	Run(ctx context.Context, args ScriptArgs) ([]int64, error)
}

// redisScriptRunner is the production implementation using go-redis.
type redisScriptRunner struct {
	rdb    *redis.Client
	script *redis.Script
}

// newRedisScriptRunner creates a script runner with a preloaded Lua script.
func newRedisScriptRunner(rdb *redis.Client) *redisScriptRunner {
	return &redisScriptRunner{
		rdb: rdb,
		script: redis.NewScript(`
			local key    = KEYS[1]
			local now    = tonumber(ARGV[1])
			local win    = tonumber(ARGV[2])
			local limit  = tonumber(ARGV[3])
			local ttl    = tonumber(ARGV[4])
			local member = ARGV[5]

			redis.call('ZREMRANGEBYSCORE', key, '-inf', win)
			local count = redis.call('ZCARD', key)

			if count < limit then
				redis.call('ZADD', key, now, member)
				redis.call('PEXPIRE', key, ttl)
				return {1, limit - count - 1}
			end

			return {0, 0}
		`),
	}
}

// Run executes the Lua script atomically in Redis.
func (r *redisScriptRunner) Run(
	ctx context.Context,
	args ScriptArgs,
) ([]int64, error) {

	return r.script.Run(ctx, r.rdb, []string{args.Key},
		args.NowNs,
		args.WinNs,
		args.Limit,
		args.TTLms,
		args.Member,
	).Int64Slice()
}
