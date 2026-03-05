package middleware

import (
	"fmt"
	"g-aigateway/pkg/logger"
	"g-aigateway/pkg/redis"
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

			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if host == "" || host == "::1" {
				host = "127.0.0.1"
			}

			key := "limit:" + host // 构造限流key

			windowSec := int(window.Seconds()) // 转换窗口时长为秒数

			val, err := redis.RDB.Eval(r.Context(), limitScript, []string{key}, limit, windowSec).Int() // 执行Redis Lua 脚本

			if err != nil {
				logger.Error("RATELIMIT", err, "Redis failure, falling back to Fail-Open mode")

				next.ServeHTTP(w, r)
				return
			}

			if val == 0 {
				logger.Info("RATELIMIT", fmt.Sprintf("Request blocked for host: %s (Current limit: %d/%ds)", host, limit, windowSec))

				w.Header().Set("Retry-After", fmt.Sprintf("%d", windowSec))
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Too Many Requests - Rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
