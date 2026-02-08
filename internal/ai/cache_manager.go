package ai

import (
	"context"
	"crypto/sha256"
	"fmt"
	"g-aigateway/pkg/redis"
	"log"
	"time"
)

type AICache struct {
	store VectorStore
}

func NewAICache(store VectorStore) *AICache {
	return &AICache{store: store}
}

// calcHash 计算字符串的 SHA256，用于精确匹配的 Key
func calcHash(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", hash)
}

// GetResponse 分级查询逻辑
func (c *AICache) GetResponse(ctx context.Context, prompt string) (string, bool) {
	// 1. Level 1: 精确匹配 (Hash) - O(1)
	// 没有任何网络请求和计算开销，速度最快
	h := calcHash(prompt)
	hashKey := "exact:" + h
	if val, err := redis.RDB.Get(ctx, hashKey).Result(); err == nil {
		log.Printf("[Cache] 精确哈希命中: %s", h)
		return val, true
	}

	// 2. Level 2: 语义匹配 (Vector) - O(N)
	// 只有精确匹配失败，才消耗 Token 调用 Embedding
	queryVec, err := GetEmbedding(prompt)
	if err != nil || queryVec == nil {
		log.Printf("[L2] 向量生成失败: %v", err)
		return "", false
	}

	res, similarity, found := c.store.Search(ctx, queryVec, 0.8)
	// 添加L2命中日志，仅在命中时打印
	if found {
		log.Printf("[L2 Hit] 语义向量命中 → 相似度: %.4f (阈值: 0.8)", similarity)
	} else {
		// 未命中时也打印最高相似度，方便调整阈值
		log.Printf("[L2 Miss] 语义向量未命中 → 最高相似度: %.4f (阈值: 0.8)", similarity)
	}
	return res, found
}

// SetResponse 同步存入两级缓存
func (c *AICache) SetResponse(ctx context.Context, prompt, response string) {
	h := calcHash(prompt)

	// 存入 L1: 精确匹配
	log.Printf("[Cache] 正在异步存入向量库...")
	redis.RDB.Set(ctx, "exact:"+h, response, 24*time.Hour)

	// 存入 L2: 向量匹配
	vec, err := GetEmbedding(prompt)
	if err != nil || vec == nil {
		return
	}

	_ = c.store.Add(ctx, VectorItem{
		ID:       h,
		Vector:   vec,
		Response: response,
		Prompt:   prompt,
	})
	log.Printf("[Cache] 向量库写入成功: %s", prompt)
}
