package proxy

import (
	"bytes"
	"io"
	"log"
	"math"
	"net/http"
	"time"
)

type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	// 为了能多次重试，我们需要缓存 Request Body
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
	}

	for i := 0; i <= t.MaxRetries; i++ {
		// 每次请求都要重新创建一个 Body 的流
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		resp, err := t.Base.RoundTrip(req)

		// 判断是否需要重试：1. 网络错误 2. 状态码为 429 或 503
		if err != nil || (resp != nil && (resp.StatusCode == 429 || resp.StatusCode >= 500)) {
			lastResp = resp
			lastErr = err

			if i < t.MaxRetries {
				// 指数退避：2^i * 100ms (100ms, 200ms, 400ms...)
				waitTime := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
				log.Printf("[Retry] 第 %d 次请求失败，等待 %v 后重试...", i+1, waitTime)
				time.Sleep(waitTime)
				continue
			}
		}
		return resp, err
	}
	return lastResp, lastErr
}
