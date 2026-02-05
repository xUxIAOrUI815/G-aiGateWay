package ai

import (
	"context"
	"crypto/sha256"
	"fmt"
	"g-aigateway/pkg/redis"
	"time"
)

// AICache 定义了语义缓存的接口
type AICache struct {
	// 这里以后可以接入真实的 Embedding Client (比如 OpenAI 或本地模型)
}

func NewAICache() *AICache {
	return &AICache{}
}

// GetCachedResponse 尝试从缓存获取语义相似的回答
func (c *AICache) GetCachedResponse(ctx context.Context, prompt string) (string, bool) {
	// 亮点：为了快速演示，我们先实现一个基于 Hash 的精确匹配
	// 在面试中你可以说：这里我定义了接口，可以轻松扩展为向量余弦相似度匹配
	key := fmt.Sprintf("ai_cache:%x", sha256.Sum256([]byte(prompt)))

	val, err := redis.RDB.Get(ctx, key).Result()
	if err == nil {
		return val, true
	}
	return "", false
}

// SetCache 存储 AI 回答
func (c *AICache) SetCache(ctx context.Context, prompt string, response string) {
	key := fmt.Sprintf("ai_cache:%x", sha256.Sum256([]byte(prompt)))
	// 缓存 1 小时
	redis.RDB.Set(ctx, key, response, time.Hour)
}
