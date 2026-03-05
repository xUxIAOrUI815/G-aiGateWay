package middleware

import (
	"g-aigateway/pkg/logger"
	"net/http"
	"time"
)

// responseWriter 包装器，用于捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		cacheHit := rw.Header().Get("X-Cache-Hit")
		if cacheHit == "" {
			cacheHit = "false"
		}

		logger.Access(r.Method, r.URL.Path, rw.statusCode, duration, cacheHit)
	})
}
