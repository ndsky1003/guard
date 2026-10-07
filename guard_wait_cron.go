package guard

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ndsky1003/lease"
)

// GuardWaitCond 是基于 sync.Cond 的限流实现。
// 与 guard_wait_sem.go 的 GuardWaitSem 各有所长：本实现不支持 context、每次固定 1 票，
// 但高竞争场景零分配。需要超时/加权请使用 GuardWaitSem。
type GuardWaitCond struct {
	l   *lease.Lease[string, *condBucket]
	ttl time.Duration
}

// condBucket 是底层共享桶：票计数 + 条件变量。
type condBucket struct {
	cap       int
	available int        // 可用票数
	cond      *sync.Cond // 条件变量
	mu        sync.Mutex // 保护 available / waiting
	waiting   int        // 等待中的 goroutine 数量
}

// BucketCond 是 GetBucket 返回的包装，持有底层共享桶与一次租约引用。
type BucketCond struct {
	b       *condBucket
	release func()
}

// NewGuardWaitCond 创建一个基于 sync.Cond 的限流管理器。
func NewGuardWaitCond(checkInterval, ttl time.Duration) *GuardWaitCond {
	if checkInterval <= 0 {
		checkInterval = time.Second
	}
	g := &GuardWaitCond{
		ttl: ttl,
	}
	g.l = lease.NewWithOptions(lease.Options[string, *condBucket]{
		Tick: checkInterval,
	})
	return g
}

// Close 停止租约时间轮，释放所有桶；重复调用安全（幂等）。
func (g *GuardWaitCond) Close() {
	if g != nil && g.l != nil {
		g.l.Stop()
	}
}

// GetBucket 获取一个指定 key 和容量的桶；相同 key 与容量复用同一个底层桶。
// 返回的 BucketCond 用完后需调用 Release 释放租约引用。
func (g *GuardWaitCond) GetBucket(key string, cap int) (*BucketCond, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	if cap <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}

	// 将容量编码进底层 key，保证相同 key 不同容量各自独立，互不复用。
	k := key + ":" + strconv.Itoa(cap)
	cb, release, err := g.l.MustGetWithGen(k, g.ttl, func(s string) (*condBucket, error) {
		b := &condBucket{
			cap:       cap,
			available: cap,
		}
		b.cond = sync.NewCond(&b.mu)
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	return &BucketCond{b: cb, release: release}, nil
}

// Release 释放本次 GetBucket 持有的租约引用；release 幂等，多次调用安全。
func (b *BucketCond) Release() {
	if b.release != nil {
		b.release()
	}
}

// GotTicket 阻塞获取一张票，直到有可用票；返回自身以支持链式调用。
func (b *BucketCond) GotTicket() *BucketCond {
	b.b.mu.Lock()
	defer b.b.mu.Unlock()

	for b.b.available <= 0 {
		b.b.waiting++
		b.b.cond.Wait()
		b.b.waiting--
	}

	b.b.available--
	return b
}

// ReleaseTicket 释放一张票，必须与一次成功的 GotTicket/TryGotTicket 成对调用。
func (b *BucketCond) ReleaseTicket() {
	b.b.mu.Lock()
	defer b.b.mu.Unlock()

	if b.b.available >= b.b.cap {
		panic("guard: released more than held")
	}
	b.b.available++

	if b.b.waiting > 0 {
		b.b.cond.Signal()
	}
}

// TryGotTicket 非阻塞尝试获取一张票；成功返回 true。
func (b *BucketCond) TryGotTicket() bool {
	b.b.mu.Lock()
	defer b.b.mu.Unlock()

	if b.b.available > 0 {
		b.b.available--
		return true
	}
	return false
}
