package middleware

import (
	"context"
	"g-aigateway/pkg/redis"
	"log"
	"net"
	"net/http"
	"time"
)

// Lua 脚本：原子性地进行 key 的自增和过期设置
var limitScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = redis.call("INCR", key)
if current == 1 then
    redis.call("EXPIRE", key, window)
end

if current > limit then
    return 0
else
    return 1
end
`

func RateLimitMiddleware(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			if host == "" || host == "::1" {
				host = "127.0.0.1"
			}

			key := "limit:" + host
			ctx := context.Background()

			// 显式打印 window 的秒数，确认不是 0
			windowSec := int(window.Seconds())

			val, err := redis.RDB.Eval(ctx, limitScript, []string{key}, limit, windowSec).Int()

			// 【关键日志】在终端看这里的 Current 变化
			log.Printf("[RateLimit] Key: %s | Count: %d/%d | Window: %ds", key, val, limit, windowSec)

			if err != nil || val == 0 {
				log.Printf("[RateLimit] !!! 拦截请求: %s", host)
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
