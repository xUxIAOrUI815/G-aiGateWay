package middleware

import (
	"context"
	"g-aigateway/pkg/redis"
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
			// 这里简单以 IP 作为限流标识
			ip := r.RemoteAddr
			ctx := context.Background()

			// 执行 Lua 脚本
			result, err := redis.RDB.Eval(ctx, limitScript, []string{"limit:" + ip}, limit, int(window.Seconds())).Int()

			if err != nil || result == 0 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("请求过于频繁，请稍后再试"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
