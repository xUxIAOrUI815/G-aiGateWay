package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"g-aigateway/internal/ai"
	"g-aigateway/pkg/logger"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type AIProxy struct {
	proxy *httputil.ReverseProxy
	cache *ai.AICache
}

// NewAIProxy 接收一个初始化好的 AICache 实例 (符合依赖注入设计)
func NewAIProxy(cache *ai.AICache) *AIProxy {
	rawURL := strings.TrimSpace(os.Getenv("TARGET_URL"))
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" {
		logger.Error("BOOT", err, "Invalid TARGET_URL: "+rawURL)
		os.Exit(1)
	}

	ap := &AIProxy{
		cache: cache,
	}

	// 1. 初始化反向代理
	p := httputil.NewSingleHostReverseProxy(target)

	// 2. 配置 Director (修改发送给上游的请求)
	originalDirector := p.Director
	p.Director = func(req *http.Request) {
		originalDirector(req)

		apiKey := os.Getenv("UPSTREAM_API_KEY")
		if apiKey == "" {
			logger.Info("PROXY", "Warning: Upstream API Key is not configured")
		}

		// 鉴权
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// 必须覆盖 Host 头
		req.Host = target.Host

		req.Header.Del("Accept-Encoding")
		logger.Info("PROXY", fmt.Sprintf("Forwarding % s %s -> %s", req.Method, req.URL.Path, target.Host))
	}

	// 配置 Transport
	p.Transport = &RetryTransport{
		Base:       http.DefaultTransport,
		MaxRetries: 3,
	}

	// 配置 ModifyResponse
	p.ModifyResponse = func(resp *http.Response) error {
		// 只有 200 成功的 JSON 响应才缓存
		if resp.StatusCode == http.StatusOK && strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			// 从 Context 中取出请求时的 Prompt
			val := resp.Request.Context().Value("current_prompt")
			prompt, ok := val.(string)
			if !ok {
				return nil
			}

			// 读取响应体
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				logger.Error("PROXY", err, "Failed to read upstream response body")
				return err
			}
			resp.Header.Del("Content-Length")
			// 必须写回 Body，否则客户端收不到数据
			resp.Body = io.NopCloser(bytes.NewBuffer(body))

			// 调用分级存储逻辑 (内部会自动处理 L1 Hash 和 L2 Vector)
			// 注意：这里使用 context.Background() 防止请求结束后 context 被取消导致写入失败
			go ap.cache.SetResponse(context.Background(), prompt, string(body))
			logger.Info("CACHE", "Response intercepted, async caching task triggered")
		}
		return nil
	}

	// 配置 ErrorHandler
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("PROXY", err, "Reverse proxy forwarding error")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "Gateway Error: %v", err)
	}

	ap.proxy = p
	return ap
}

func (ap *AIProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 仅对 POST 请求尝试检索缓存 (通常 AI Chat 接口都是 POST)
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("PROXY", err, "Failed to read client request body")
			http.Error(w, "Request Body Error", http.StatusBadRequest)
			return
		}
		// 重新填充 Body 供代理转发读取
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		var reqMap map[string]interface{}
		if err := json.Unmarshal(body, &reqMap); err == nil {
			// 将提问内容转为字符串作为检索标识
			prompt := fmt.Sprintf("%v", reqMap["messages"])

			// 将 Prompt 存入 Context，以便后续在 ModifyResponse 中能取出来作为 Cache Key
			r = r.WithContext(context.WithValue(r.Context(), "current_prompt", prompt))

			// 执行分级查询：L1(Hash) -> L2(Vector)
			if val, ok := ap.cache.GetResponse(r.Context(), prompt); ok {
				w.Header().Set("X-Cache-Hit", "true")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write([]byte(val))
				return
			}
		}
	}

	// 缓存未命中，执行反向代理转发
	ap.proxy.ServeHTTP(w, r)
}
