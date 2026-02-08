package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"g-aigateway/internal/ai"
	"g-aigateway/internal/middleware"
	"g-aigateway/internal/proxy"
	"g-aigateway/pkg/redis"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	redis.InitRedis()

	// 【新增：强制清空 Redis】确保实验开始时没有任何旧数据
	log.Println("[Debug] 正在强制清空 Redis 缓存...")
	redis.RDB.FlushAll(context.Background())

	store := &ai.RedisVectorStore{}
	cacheManager := ai.NewAICache(store)
	aiProxy := proxy.NewAIProxy(cacheManager)

	// 【修正：显式指定 time.Second】
	// 之前你可能只传了 60，导致过期时间太短。
	limiter := middleware.RateLimitMiddleware(5, 60*time.Second)

	handler := middleware.LoggerMiddleware(limiter(http.HandlerFunc(aiProxy.ServeHTTP)))

	log.Printf("[Boot] G-AIGateway 启动在 :8080")
	http.ListenAndServe(":8080", handler)
}
