package proxy

import (
	"bytes"
	"fmt"
	"g-aigateway/pkg/logger"
	"io"
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

	// 缓存请求体
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
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
				if resp != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}

				waitTime := time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
				logger.Info("RETRY", fmt.Sprintf("Attempt %d failed. Waiting %v before retry...", i+1, waitTime))

				select {
				case <-time.After(waitTime):
					continue
				case <-req.Context().Done():
					logger.Info("RETRY", "Request canceled by client, stopping retries")
					return lastResp, req.Context().Err()
				}
			}
		} else {
			return resp, err
		}
	}

	if lastErr != nil {
		logger.Error("RETRY", lastErr, fmt.Sprintf("All %d retries exhausted", t.MaxRetries))
	}

	return lastResp, lastErr
}
