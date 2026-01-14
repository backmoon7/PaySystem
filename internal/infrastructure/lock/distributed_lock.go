package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// ============================================================================
// 分布式锁实现
// ============================================================================
//
// 【为什么需要分布式锁？】
//
// 场景：用户A同时发起两笔支付请求（比如网络抖动导致重复提交）
//
// 如果没有分布式锁：
//   goroutine1: 查询余额=100 -> 扣款100 -> 余额=0   OK
//   goroutine2: 查询余额=100 -> 扣款100 -> 余额=-100 超扣了！
//
// 加了分布式锁：
//   goroutine1: 获取锁 -> 查询余额=100 -> 扣款100 -> 余额=0 -> 释放锁
//   goroutine2: 获取锁失败，等待... -> 获取锁 -> 查询余额=0 -> 余额不足，拒绝
//
// 【Redis 分布式锁原理】
//
// 加锁：SET key value NX EX timeout
//   - NX: 只有 key 不存在时才设置（保证互斥）
//   - EX: 设置过期时间（防止死锁）
//   - value: 锁持有者标识（释放时验证，防止误删别人的锁）
//
// 释放锁：使用 Lua 脚本保证原子性
//   - 先检查 value 是否是自己的
//   - 再删除 key
//
// ============================================================================

var (
	ErrLockFailed  = errors.New("获取分布式锁失败")
	ErrLockExpired = errors.New("锁已过期")
)

// DistributedLock 分布式锁
type DistributedLock struct {
	client     *redis.Client
	key        string        // 锁的 key
	value      string        // 锁的 value（用于验证锁的持有者）
	expiration time.Duration // 锁的过期时间
}

// NewDistributedLock 创建分布式锁
func NewDistributedLock(client *redis.Client, key, value string, expiration time.Duration) *DistributedLock {
	return &DistributedLock{
		client:     client,
		key:        key,
		value:      value,
		expiration: expiration,
	}
}

// TryLock 尝试获取锁（非阻塞）
//
// 【关键点】使用 SetNX 命令，只有当 key 不存在时才能设置成功
// 这保证了同一时刻只有一个客户端能获取到锁
func (l *DistributedLock) TryLock(ctx context.Context) (bool, error) {
	// SET key value NX EX timeout
	// NX: 只有 key 不存在时才设置
	// EX: 设置过期时间，防止死锁（持有锁的进程崩溃时，锁会自动释放）
	success, err := l.client.SetNX(ctx, l.key, l.value, l.expiration).Result()
	if err != nil {
		return false, err
	}
	return success, nil
}

// Lock 阻塞式获取锁（带重试）
func (l *DistributedLock) Lock(ctx context.Context, retryInterval time.Duration, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		success, err := l.TryLock(ctx)
		if err != nil {
			return err
		}
		if success {
			return nil
		}
		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryInterval):
			// 继续重试
		}
	}
	return ErrLockFailed
}

// Unlock 释放锁
//
// 【关键点】使用 Lua 脚本保证"检查+删除"操作的原子性
//
// 为什么要检查 value？
//
//	场景：A 获取锁 -> A 处理超时，锁自动过期 -> B 获取锁 -> A 执行完毕，调用 Unlock
//	如果不检查 value，A 会把 B 的锁删掉！
//
//	使用 value 验证后：
//	A 的 Unlock 发现 value 不是自己的，不会删除，B 的锁安全
func (l *DistributedLock) Unlock(ctx context.Context) error {
	// Lua 脚本：检查 value 是否匹配，匹配则删除
	// 使用 Lua 脚本保证原子性，避免"检查-删除"之间的并发问题
	script := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`
	_, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	return err
}

// ============================================================================
// 便捷函数：基于用户ID的支付锁
// ============================================================================

// NewPayLock 创建支付锁（按用户维度）
//
// 【设计思考】为什么按用户维度加锁？
//
// 方案1：全局锁（所有用户共用一把锁）
//   - 优点：实现简单
//   - 缺点：并发度极低，用户A支付时，用户B也要等待
//
// 方案2：按用户加锁（每个用户独立一把锁）  <-- 我们的选择
//   - 优点：不同用户可以并发支付
//   - 缺点：同一用户不能并发（这正是我们想要的！）
//
// 方案3：按账户+金额加锁（更细粒度）
//   - 在热点账户场景下使用，当前项目暂不需要
func NewPayLock(client *redis.Client, userID int64, requestID string) *DistributedLock {
	key := fmt.Sprintf("pay:lock:user:%d", userID)
	// value 使用 requestID，便于追踪是哪个请求持有锁
	return NewDistributedLock(client, key, requestID, 30*time.Second)
}
