package guard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

// Bucket 现在内部使用 semaphore.Weighted 来控制并发
type Bucket struct {
	sem     *semaphore.Weighted
	lastUse time.Time
	cap     int64
}

// Acquire 尝试获取一个许可。
// 它会阻塞直到成功获取、或 context 被取消/超时。
func (b *Bucket) Acquire(ctx context.Context) error {
	// 请求权重为 1 的信号量
	if err := b.sem.Acquire(ctx, 1); err != nil {
		return err // 如果 context 被取消或超时，这里会返回错误
	}
	b.lastUse = time.Now()
	return nil
}

// Release 释放一个许可
func (b *Bucket) Release() {
	b.lastUse = time.Now()
	b.sem.Release(1)
}

// GuardWait 管理一组命名的 Bucket
type GuardWait struct {
	m              map[string]*Bucket
	l              sync.Mutex
	bucketLifeTime time.Duration

	// 用于停止 GC goroutine
	stopGc chan struct{}
	wg     sync.WaitGroup
}

// NewGuardWait 创建一个新的 GuardWait 管理器。
/*
checkInterval 检查间隔，是否有不用了的桶
bucketLifeTime 桶多久不用就释放掉
*/
func NewGuardWait(checkInterval time.Duration, bucketLifeTime time.Duration) *GuardWait {
	g := &GuardWait{
		m:              make(map[string]*Bucket),
		bucketLifeTime: bucketLifeTime,
		stopGc:         make(chan struct{}),
	}

	if checkInterval > 0 && bucketLifeTime > 0 {
		g.wg.Add(1)
		go g.gcLoop(checkInterval)
	}

	return g
}

// Close 会优雅地停止 GC goroutine，防止泄露
func (g *GuardWait) Close() {
	close(g.stopGc)
	g.wg.Wait()
}

func (g *GuardWait) gcLoop(interval time.Duration) {
	defer g.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			g.gc()
		case <-g.stopGc:
			return
		}
	}
}

func (g *GuardWait) gc() {
	g.l.Lock()
	defer g.l.Unlock()

	now := time.Now()
	for k, b := range g.m {
		// 如果桶过期，并且当前无人使用，则回收
		if b.lastUse.Add(g.bucketLifeTime).Before(now) {
			// TryAcquire 会尝试获取所有许可，如果成功说明当前无人持有许可
			// 这是一个技巧，用来检查信号量是否“空闲”
			if b.sem.TryAcquire(b.cap) { // 假设 cap 信息需要被存储或可计算
				fmt.Println("GuardWait: gc collected bucket:", k)
				delete(g.m, k)
				// 注意：这里没有 Release，因为我们就是要销毁它
			}
		}
	}
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

	g.l.Lock()
	defer g.l.Unlock()

	if b, ok := g.m[key]; ok {
		b.lastUse = time.Now()
		return b, nil
	}

	b := &Bucket{
		sem:     semaphore.NewWeighted(cap),
		lastUse: time.Now(),
		cap:     cap,
	}
	g.m[key] = b
	return b, nil
}
