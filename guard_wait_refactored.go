package guard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ndsky1003/lease"
	"golang.org/x/sync/semaphore"
)

// Bucket 内部使用 semaphore.Weighted 控制并发
type Bucket struct {
	sem   *semaphore.Weighted
	touch func() // 记录一次访问，续期租约
}

// Acquire 尝试获取一个许可。
// 它会阻塞直到成功获取、或 context 被取消/超时。
func (b *Bucket) Acquire(ctx context.Context) error {
	if err := b.sem.Acquire(ctx, 1); err != nil {
		return err // 如果 context 被取消或超时，这里会返回错误
	}
	if b.touch != nil {
		b.touch()
	}
	return nil
}

// Release 释放一个许可
func (b *Bucket) Release() {
	b.sem.Release(1)
	if b.touch != nil {
		b.touch()
	}
}

// GuardWait 管理一组命名的 Bucket
type GuardWait struct {
	l              *lease.Lease[string, *Bucket]
	mu             sync.Mutex
	bucketLifeTime time.Duration
}

// NewGuardWait 创建一个新的 GuardWait 管理器。
/*
checkInterval 租约到期检查粒度（时间轮 tick）
bucketLifeTime 桶多久不用就释放掉
*/
func NewGuardWait(checkInterval, bucketLifeTime time.Duration) *GuardWait {
	if checkInterval <= 0 {
		checkInterval = time.Second
	}
	g := &GuardWait{
		bucketLifeTime: bucketLifeTime,
	}
	g.l = lease.NewWithOptions(lease.Options[string, *Bucket]{
		Tick: checkInterval,
	})
	return g
}

// Close 停止租约时间轮，释放所有桶
func (g *GuardWait) Close() {
	g.l.Stop()
}

// GetBucket 获取一个指定 key 和容量的 Bucket。
// 如果 Bucket 不存在，会自动创建。
func (g *GuardWait) GetBucket(key string, cap int64) (*Bucket, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty") // 返回 error 而不是 panic
	}
	if cap <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if b, release, ok := g.l.Get(key); ok {
		defer release()
		return b, nil
	}

	b := &Bucket{
		sem: semaphore.NewWeighted(cap),
	}
	b.touch = func() {
		_, release, ok := g.l.Get(key)
		if ok {
			release()
		}
	}
	g.l.Set(key, b, g.bucketLifeTime)
	return b, nil
}
