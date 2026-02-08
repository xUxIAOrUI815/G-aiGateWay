package ai

import (
	"context"
	"encoding/binary"
	"g-aigateway/pkg/redis"
	"log"
	"math"
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

	// 执行 Redis 写入
	return redis.RDB.HSet(ctx, key, data).Err()
}

// Search 必须完全匹配接口定义: (ctx context.Context, queryVector []float32, threshold float32) (string, bool)
func (s *RedisVectorStore) Search(ctx context.Context, queryVec []float32, threshold float32) (string, float32, bool) {
	var bestResponse string
	var bestSimilarity float32 = 0.0 // 初始化为0
	var found bool = false

	// 获取所有以 vec: 开头的 Key
	keys, err := redis.RDB.Keys(ctx, "vec:*").Result()
	if err != nil || len(keys) == 0 {
		log.Printf("[RedisVectorSearch] 未找到任何向量Key，直接返回未命中")
		return "", 0.0, false
	}

	log.Printf("[RedisVectorSearch] 找到 %d 个向量Key，开始逐一计算相似度", len(keys))

	// 遍历所有向量Key
	for _, key := range keys {
		data, err := redis.RDB.HGetAll(ctx, key).Result()
		if err != nil {
			log.Printf("[RedisVectorSearch] 读取Key %s 失败: %v，跳过", key, err)
			continue
		}

		// 取出二进制向量并转回 []float32
		storedVecBinary := []byte(data["vec"])
		if len(storedVecBinary) == 0 {
			log.Printf("[RedisVectorSearch] Key %s 的向量数据为空，跳过", key)
			continue
		}
		storedVec := ByteToFloat32(storedVecBinary)

		// 计算余弦相似度
		similarity := CosineSimilarity(queryVec, storedVec)

		prompt := data["prompt"] // 假设你存向量时同步存了prompt字段（SetResponse中补充）
		log.Printf("[RedisVectorSearch] 对比Key: %s → 提问: %s → 相似度: %.4f (阈值: %.1f)",
			key, prompt, similarity, threshold)

		if similarity >= threshold {
			if similarity > bestSimilarity {
				bestSimilarity = similarity
				bestResponse = data["res"]
				found = true
			}
		}
	}
	if found {
		log.Printf("[RedisVectorSearch] 命中！最高相似度: %.4f (阈值: %.1f) → 响应: %s",
			bestSimilarity, threshold, bestResponse[:min(len(bestResponse), 50)]) // 截断长响应，避免日志刷屏
	} else {
		log.Printf("[RedisVectorSearch] 未命中！最高相似度: %.4f (阈值: %.1f)",
			bestSimilarity, threshold)
	}
	return bestResponse, bestSimilarity, found
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 辅助函数：Float32ToByte
func Float32ToByte(slice []float32) []byte {
	b := make([]byte, len(slice)*4)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

// 辅助函数：ByteToFloat32
func ByteToFloat32(b []byte) []float32 {
	l := len(b) / 4
	res := make([]float32, l)
	for i := 0; i < l; i++ {
		res[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4 : (i+1)*4]))
	}
	return res
}
