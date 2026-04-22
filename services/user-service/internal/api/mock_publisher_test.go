package api

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// stubPublisher returns a *Publisher with a dummy redis client. Publish
// calls will fail with connection errors that get logged and swallowed
// (handlers ignore publish errors), avoiding nil panics. DialTimeout is
// 1ms so tests don't block on connection attempts.
func stubPublisher() *Publisher {
	rdb := redis.NewClient(&redis.Options{
		Addr:        "localhost:0",
		DialTimeout: time.Millisecond,
		MaxRetries:  0,
	})
	return &Publisher{rdb: rdb}
}
