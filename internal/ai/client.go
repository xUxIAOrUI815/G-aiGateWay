package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// GetEmbedding 调用 API 获取文本的向量表示
func GetEmbedding(text string) ([]float32, error) {
	apiKey := os.Getenv("EMBEDDING_API_KEY")
	url := os.Getenv("EMBEDDING_URL")
	model := os.Getenv("EMBEDDING_MODEL")

	payload, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"input": text,
	})

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	log.Printf("[AI] 正在获取向量 Embedding...")
	// resp, err := http.DefaultClient.Do(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result EmbeddingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		return result.Data[0].Embedding, nil
	}

	return nil, nil
}
