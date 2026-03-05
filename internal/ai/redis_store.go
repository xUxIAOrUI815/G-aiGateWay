package ai

import (
	"context"
	"encoding/binary"
	"fmt"
	"g-aigateway/pkg/logger"
	"g-aigateway/pkg/redis"
	"math"
	"time"
)

// 确保指针类型 *RedisVectorStore 实现了 VectorStore 接口
var _ VectorStore = (*RedisVectorStore)(nil)

type RedisVectorStore struct{}

// Add 必须完全匹配接口定义: (ctx context.Context, item VectorItem) error
func (s *RedisVectorStore) Add(ctx context.Context, item VectorItem) error {
	key := "vec:" + item.ID

	// 将向量转为二进制 []byte
	vecBinary := Float32ToByte(item.Vector)

	// 使用 Hash 存储，字段名要和 Search 保持一致
	data := map[string]interface{}{
		"vec": vecBinary,
		"res": item.Response,
		"p":   item.Prompt,
	}

	// 1. 写入哈希数据
	if err := redis.RDB.HSet(ctx, key, data).Err(); err != nil {
		logger.Error("STORAGE", err, "Failed to HSet vector data for key:"+key)
		return err
	}

	// 2. 设置 TTL
	if err := redis.RDB.Expire(ctx, key, 48*time.Hour).Err(); err != nil {
		logger.Error("STORAGE", err, "Failed to set TTL for key: "+key)
		return err
	}

	return nil
}

// Search 遍历 Redis 向量 Key 进行余弦相似度计算
func (s *RedisVectorStore) Search(ctx context.Context, queryVec []float32, threshold float32) (string, float32, bool) {
	var bestResponse string
	var bestSimilarity float32 = 0.0
	var found bool = false

	// 获取所有以 vec: 开头的 Key
	keys, err := redis.RDB.Keys(ctx, "vec:*").Result()
	if err != nil {
		logger.Error("STORAGE", err, "Failed to scan vector keys")
		return "", 0.0, false
	}

	if len(keys) == 0 {
		return "", 0.0, false
	}

	// 遍历所有向量Key
	for _, key := range keys {
		data, err := redis.RDB.HGetAll(ctx, key).Result()
		if err != nil {
			logger.Error("STORAGE", err, "Failed to read hash key: "+key)
			continue
		}

		// 取出二进制向量并转回 []float32
		storedVecBinary := []byte(data["vec"])
		if len(storedVecBinary) == 0 {
			continue
		}
		storedVec := ByteToFloat32(storedVecBinary)

		// 计算余弦相似度
		similarity := CosineSimilarity(queryVec, storedVec)

		originalPrompt := data["p"]
		logger.Info("VECTOR-SCAN", fmt.Sprintf("Compare: %s | Sim: %.4f | Prompt: %s",
			key[:12], similarity, truncate(originalPrompt, 30)))

		if similarity >= threshold {
			if similarity > bestSimilarity {
				bestSimilarity = similarity
				bestResponse = data["res"]
				found = true
			}
		}
	}

	return bestResponse, bestSimilarity, found
}

// 辅助工具函数
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func Float32ToByte(slice []float32) []byte {
	b := make([]byte, len(slice)*4)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}
func ByteToFloat32(b []byte) []float32 {
	l := len(b) / 4
	res := make([]float32, l)
	for i := 0; i < l; i++ {
		res[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4 : (i+1)*4]))
	}
	return res
}
