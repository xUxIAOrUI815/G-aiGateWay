package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"g-aigateway/internal/ai"
	"io"
	"log"
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

func NewAIProxy() *AIProxy {
	rawURL := strings.TrimSpace(os.Getenv("TARGET_URL"))
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" {
		log.Fatalf("错误: 无法解析 TARGET_URL [%s]: %v", rawURL, err)
	}

	ap := &AIProxy{
		cache: ai.NewAICache(),
	}

	// 核心修复点：使用 NewSingleHostReverseProxy 确保基础转发逻辑（Scheme/Host）被正确初始化
	p := httputil.NewSingleHostReverseProxy(target)

	// 保存原始的 Director (它负责设置请求的 Scheme 和 Host)
	originalDirector := p.Director

	p.Director = func(req *http.Request) {
		// 1. 先调用原始逻辑，确保 req.URL.Scheme 和 req.URL.Host 被填上
		originalDirector(req)

		// 2. 注入鉴权 Header
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// 3. 必须覆盖 req.Host，否则上游服务器可能会拒绝请求
		req.Host = target.Host

		log.Printf("[Proxy] 转发请求到: %s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.Path)
	}

	// 集成重试机制
	p.Transport = &RetryTransport{
		Base:       http.DefaultTransport,
		MaxRetries: 3,
	}

	// 拦截响应并存入缓存
	p.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == http.StatusOK {
			val := resp.Request.Context().Value("current_prompt")
			prompt, ok := val.(string)
			if ok && resp.Header.Get("Content-Type") == "application/json" {
				body, _ := io.ReadAll(resp.Body)
				resp.Body = io.NopCloser(bytes.NewBuffer(body))
				ap.cache.SetCache(context.Background(), prompt, string(body))
				log.Printf("[Cache] 响应已存入缓存")
			}
		}
		return nil
	}

	// 错误处理：捕获转发过程中的 panic 或网络错误
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] 转发失败: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "网关错误: %v", err)
	}

	ap.proxy = p
	return ap
}

func (ap *AIProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "读取请求体失败", http.StatusBadRequest)
			return
		}
		// 重新填充 Body 供代理后续读取
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		var reqMap map[string]interface{}
		if err := json.Unmarshal(body, &reqMap); err == nil {
			// 将提问内容转为字符串作为 Cache Key
			prompt := fmt.Sprintf("%v", reqMap["messages"])

			// 存入 Context 供后续使用
			r = r.WithContext(context.WithValue(r.Context(), "current_prompt", prompt))

			// 尝试命中缓存
			if val, ok := ap.cache.GetCachedResponse(r.Context(), prompt); ok {
				log.Printf("[Cache] 命中语义缓存！")
				w.Header().Set("X-Cache-Hit", "true")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(val))
				return
			}
		}
	}

	ap.proxy.ServeHTTP(w, r)
}
