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

// NewAIProxy 接收一个初始化好的 AICache 实例 (符合依赖注入设计)
func NewAIProxy(cache *ai.AICache) *AIProxy {
	rawURL := strings.TrimSpace(os.Getenv("TARGET_URL"))
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" {
		log.Fatalf("错误: 无法解析 TARGET_URL [%s]: %v", rawURL, err)
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

		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// 必须覆盖 Host 头，防止上游 CDN 或 Web 服务器拒绝访问
		req.Host = target.Host

		log.Printf("[Proxy] 转发请求: %s %s -> %s", req.Method, req.URL.Path, target.Host)
	}

	// 3. 配置 Transport (集成重试机制)
	p.Transport = &RetryTransport{
		Base:       http.DefaultTransport,
		MaxRetries: 3,
	}

	// 4. 配置 ModifyResponse (拦截响应并异步存入分级缓存)
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
				return err
			}
			// 必须写回 Body，否则客户端收不到数据
			resp.Body = io.NopCloser(bytes.NewBuffer(body))

			// 调用分级存储逻辑 (内部会自动处理 L1 Hash 和 L2 Vector)
			// 注意：这里使用 context.Background() 防止请求结束后 context 被取消导致写入失败
			go ap.cache.SetResponse(context.Background(), prompt, string(body))
			log.Printf("[Cache] 响应已交给后台存入分级缓存")
		}
		return nil
	}

	// 5. 配置 ErrorHandler
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy Error] %v", err)
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "网关错误: %v", err)
	}

	ap.proxy = p
	return ap
}

func (ap *AIProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 仅对 POST 请求尝试检索缓存 (通常 AI Chat 接口都是 POST)
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "无法读取请求体", http.StatusBadRequest)
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
				log.Printf("[Cache Hit] 成功通过分级检索命中缓存")
				w.Header().Set("X-Cache-Hit", "true")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(val))
				return
			}
		}
	}

	// 缓存未命中，执行反向代理转发
	ap.proxy.ServeHTTP(w, r)
}
