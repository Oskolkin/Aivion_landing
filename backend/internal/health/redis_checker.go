package health

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

type RedisChecker struct {
	client *goredis.Client
}

func NewRedisChecker(client *goredis.Client) *RedisChecker {
	return &RedisChecker{
		client: client,
	}
}

func (c *RedisChecker) Check(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
