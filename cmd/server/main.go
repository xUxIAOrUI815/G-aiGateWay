package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"g-aigateway/internal/ai"
	"g-aigateway/internal/middleware"
	"g-aigateway/internal/proxy"
	"g-aigateway/pkg/logger"
	"g-aigateway/pkg/redis"

	"github.com/joho/godotenv"
)

func main() {
	logger.Boot("Starting G-AIGateway service...")

	if err := godotenv.Load(); err != nil {
		logger.Info("CONFIG", "No .env file found")
	} else {
		logger.Info("CONFIG", "Configuration loaded from .env file")
	}

	logger.Boot("Initializing Redis storage...")
	if err := redis.InitRedis(); err != nil {
		logger.Error("BOOT", err, "Failed to connect to Redis")
		os.Exit(1)
	}
	logger.Boot("Redis connection established")

	logger.Info("DEBUG", "Forcing flush of all Redis keys for clean test environment")
	if err := redis.RDB.FlushAll(context.Background()).Err(); err != nil {
		logger.Error("DEBUG", err, "Failed to flush Redis keys")
	}

	// 依赖注入
	store := &ai.RedisVectorStore{}
	cacheManager := ai.NewAICache(store)
	aiProxy := proxy.NewAIProxy(cacheManager)

	// 路由配置
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fileServer)

	limitVal := 5
	window := 60 * time.Second

	// 核心业务 Handler
	businessHandler := http.HandlerFunc(aiProxy.ServeHTTP)

	// 包装限流中间件
	apiHandler := middleware.LoggerMiddleware(
		middleware.RateLimitMiddleware(limitVal, window)(businessHandler),
	)

	// 最外层包装日志中间件
	// finalHandler := middleware.LoggerMiddleware(limitedHandler)

	mux.Handle("/v1/", apiHandler)
	// 启动Http server
	port := ":8080"
	logger.Boot("G-AIGateway service started on port " + port)
	logger.Boot("Web Dashboard available at http://localhost" + port)

	server := &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error("SYSTEM", err, "Server exited unexpectedly")
		os.Exit(1)
	}
}
