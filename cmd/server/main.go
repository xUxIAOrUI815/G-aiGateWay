package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"g-aigateway/internal/middleware"
	"g-aigateway/internal/proxy"
	"g-aigateway/pkg/redis"

	"github.com/joho/godotenv"
)

func main() {
	// godotenv.Load()

	err := godotenv.Load()
	if err != nil {
		log.Printf("警告: 未找到 .env 文件，将尝试读取系统环境变量")
	}

	targetURL := os.Getenv("TARGET_URL")
	apiKey := os.Getenv("DEEPSEEK_API_KEY")

	// 调试日志：看看读到没
	log.Printf("加载配置: TARGET_URL=%s", targetURL)
	if apiKey == "" {
		log.Println("警告: DEEPSEEK_API_KEY 为空！")
	}

	if targetURL == "" {
		log.Fatal("错误: TARGET_URL 不能为空，请检查 .env 文件")
	}

	// 1. 初始化 Redis
	if err := redis.InitRedis(); err != nil {
		log.Fatalf("Redis 连接失败: %v", err)
	}
	log.Println("Redis 已连接")

	// 2. 初始化代理
	aiProxy := proxy.NewAIProxy()

	// 3. 构建中间件链
	// 限流策略：每个 IP 每分钟允许 5 次请求（测试用，你可以调大）
	limiter := middleware.RateLimitMiddleware(5, 60*time.Second)

	// 4. 将代理包装在中间件中
	finalHandler := limiter(http.HandlerFunc(aiProxy.ServeHTTP))

	log.Printf("G-AIGateway 启动在 :8080")
	http.Handle("/v1/", finalHandler)

	http.ListenAndServe(":8080", nil)
}
