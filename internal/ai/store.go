package ai

import (
	"context"
)

type VectorItem struct {
	ID       string    // Prompt 的 Hash
	Vector   []float32 // 二进制存储
	Response string    // AI 回答
	Prompt   string    // 原提问
}

type VectorStore interface {
	// Search 寻找最相似的向量，返回响应内容和是否命中
	Search(ctx context.Context, queryVector []float32, threshold float32) (string, float32, bool)
	// Add 存入新向量
	Add(ctx context.Context, item VectorItem) error
}
