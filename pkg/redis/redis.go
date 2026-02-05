package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func InitRedis() error {
	RDB = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	return RDB.Ping(context.Background()).Err()
}
