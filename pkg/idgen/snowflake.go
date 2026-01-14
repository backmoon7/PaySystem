package idgen

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================================================
// 雪花算法 ID 生成器
// ============================================================================
//
// 【为什么需要分布式ID？】
//
// 订单号要求：
//   1. 全局唯一 - 不能重复
//   2. 趋势递增 - 便于数据库索引
//   3. 高性能 - 支持高并发生成
//   4. 信息隐藏 - 不暴露业务量
//
// 【雪花算法结构】64位
//
//   0 - 41位时间戳 - 10位机器ID - 12位序列号
//   |   |            |            |
//   |   |            |            +-- 同一毫秒内的序列号（0-4095）
//   |   |            +-- 机器ID（0-1023）
//   |   +-- 毫秒级时间戳（可用约69年）
//   +-- 符号位，始终为0
//
// ============================================================================

const (
	epoch          = int64(1704067200000) // 起始时间戳（2024-01-01 00:00:00 UTC）
	workerIDBits   = 10                   // 机器ID位数
	sequenceBits   = 12                   // 序列号位数
	maxWorkerID    = -1 ^ (-1 << workerIDBits)
	maxSequence    = -1 ^ (-1 << sequenceBits)
	workerIDShift  = sequenceBits
	timestampShift = sequenceBits + workerIDBits
)

// Snowflake 雪花算法ID生成器
type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	workerID  int64
	sequence  int64
}

var (
	defaultGenerator *Snowflake
	once             sync.Once
)

// Init 初始化默认ID生成器
func Init(workerID int64) {
	once.Do(func() {
		if workerID < 0 || workerID > maxWorkerID {
			log.Fatalf("workerID 必须在 0-%d 之间", maxWorkerID)
		}
		defaultGenerator = &Snowflake{
			workerID:  workerID,
			timestamp: 0,
			sequence:  0,
		}
	})
}

// NextID 生成下一个ID
func NextID() int64 {
	if defaultGenerator == nil {
		Init(1) // 默认使用 workerID = 1
	}
	return defaultGenerator.Generate()
}

// Generate 生成ID
func (s *Snowflake) Generate() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == s.timestamp {
		// 同一毫秒内，序列号递增
		s.sequence = (s.sequence + 1) & maxSequence
		if s.sequence == 0 {
			// 序列号用完，等待下一毫秒
			for now <= s.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		// 不同毫秒，序列号重置
		s.sequence = 0
	}

	s.timestamp = now

	// 组装ID
	id := ((now - epoch) << timestampShift) |
		(s.workerID << workerIDShift) |
		s.sequence

	return id
}

// GenerateOrderNo 生成订单号
// 格式：PAY + 年月日时分秒 + 雪花ID后8位
// 例如：PAY20240115143052_12345678
func GenerateOrderNo() string {
	id := NextID()
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("PAY%s%08d", timestamp, id%100000000)
}

// GenerateTransactionNo 生成流水号
func GenerateTransactionNo() string {
	id := NextID()
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("TXN%s%08d", timestamp, id%100000000)
}

// GenerateRefundNo 生成退款单号
func GenerateRefundNo() string {
	id := NextID()
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("REF%s%08d", timestamp, id%100000000)
}
