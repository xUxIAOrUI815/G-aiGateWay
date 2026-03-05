package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"g-aigateway/pkg/logger"
	"io"
	"net/http"
	"os"
	"time"
)

// 定义 Embedding API 响应的 JSON 结构
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// GetEmbedding 调用 API 获取文本的向量表示
func GetEmbedding(text string) ([]float32, error) {
	// 从环境变量读取配置
	apiKey := os.Getenv("EMBEDDING_API_KEY")
	url := os.Getenv("EMBEDDING_URL")
	model := os.Getenv("EMBEDDING_MODEL")

	if apiKey == "" {
		err := fmt.Errorf("EMBEDDING_API_KEY environment variable is empty")
		logger.Error("AI-EMBEDDING", err, "Missing API key")
		return nil, err
	}
	if url == "" {
		err := fmt.Errorf("EMBEDDING_URL environment variable is empty")
		logger.Error("AI-EMBEDDING", err, "Missing API URL")
		return nil, err
	}
	if model == "" {
		err := fmt.Errorf("EMBEDDING_MODEL environment variable is empty")
		logger.Error("AI-EMBEDDING", err, "Missing model name")
		return nil, err
	}

	// 构造 API 请求体
	payload, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"input": text,
	})

	// 设置超时控制 避免下游API响应过慢拖垮网关
	client := &http.Client{Timeout: 15 * time.Second}
	// 构造 POST 请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("AI-EMBEDDING", err, "Failed to create request")
		return nil, err
	}

	// 设置请求头 API 要求的鉴权和内容类型
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	logger.Info("AI", fmt.Sprintf("Requesting embedding from model: %s", model))

	// 发送 HTTP 请求 执行API调用
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("AI-EMBEDDING", err, "API request failed")
		return nil, err
	}
	defer resp.Body.Close()

	// 状态码校验
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("status code: %d", resp.StatusCode)
		logger.Error("AI-EMBEDDING", err, "Upstream returned error: "+string(body))
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("AI-EMBEDDING", err, "Failed to read response body")
		return nil, err
	}

	var result EmbeddingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Error("AI-EMBEDDING", err, "JSON unmarshal failed")
		return nil, err
	}

	if len(result.Data) > 0 {
		logger.Info("AI", "Embedding request successful")
		return result.Data[0].Embedding, nil
	}

	return nil, fmt.Errorf("empty embedding data returned")
}
