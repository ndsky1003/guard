package guard

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ndsky1003/lease"
	"golang.org/x/sync/semaphore"
)

// Sem 内部使用 semaphore.Weighted 控制并发。
// 一个 Sem 由 GetSem 返回，内部持有底层信号量与一次租约引用。
// 使用方式：GetSem 后立即 defer Release 释放租约引用，
// 然后 Acquire 获取许可，用完 defer ReleaseTicket 释放许可。
type Sem struct {
	sem     *semaphore.Weighted
	release func()
	cap     int64
}

// Acquire 尝试获取 n 个许可，阻塞直到成功获取或 context 被取消/超时。
// 成功返回 nil；失败返回错误（此时未占用任何许可）。
// 租约引用的释放由调用方负责（GetSem 后 defer Release）。
func (b *Sem) Acquire(ctx context.Context, n int64) error {
	if n <= 0 {
		return fmt.Errorf("n must be positive")
	}
	if n > b.cap {
		return fmt.Errorf("n %d exceeds capacity %d", n, b.cap)
	}
	return b.sem.Acquire(ctx, n)
}

// Release 释放本次 GetSem 持有的租约引用，允许信号量在空闲后回收。
// 应在 GetSem 后立即 defer；release 幂等，多次调用安全。
// 许可的释放由 ReleaseTicket 负责。
func (b *Sem) Release() {
	if b.release != nil {
		b.release()
	}
}

// ReleaseTicket 释放 n 个许可，必须与一次成功的 Acquire 成对调用。
func (b *Sem) ReleaseTicket(n int64) {
	if n <= 0 {
		panic("sem: ReleaseTicket n must be positive")
	}
	b.sem.Release(n)
}

// GuardWaitSem 管理一组命名的信号量
type GuardWaitSem struct {
	l   *lease.Lease[string, *semaphore.Weighted]
	ttl time.Duration
}

// NewGuardWaitSem 创建一个新的 GuardWaitSem 管理器。
/*
checkInterval 租约到期检查粒度（时间轮 tick）
ttl 信号量多久不用就释放掉
*/
func NewGuardWaitSem(checkInterval, ttl time.Duration) *GuardWaitSem {
	if checkInterval <= 0 {
		checkInterval = time.Second
	}
	g := &GuardWaitSem{
		ttl: ttl,
	}
	g.l = lease.NewWithOptions(lease.Options[string, *semaphore.Weighted]{
		Tick:          checkInterval,
		RenewInterval: 1 * time.Minute,
	})
	return g
}

// Close 停止租约时间轮，释放所有信号量；重复调用安全（幂等）。
func (g *GuardWaitSem) Close() {
	if g != nil && g.l != nil {
		g.l.Stop()
	}
}

// GetSem 获取一个指定 key 和容量的 Sem。
// 相同 key 与容量会复用同一个底层信号量；Sem 不存在时自动创建。
func (g *GuardWaitSem) GetSem(key string, cap int64) (*Sem, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	if cap <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}

	// 将容量编码进底层 key，保证相同 key 不同容量各自独立，互不复用信号量。
	k := key + ":" + strconv.FormatInt(cap, 10)
	sem, release, err := g.l.MustGetWithGen(k, g.ttl, func(s string) (*semaphore.Weighted, error) {
		return semaphore.NewWeighted(cap), nil
	})
	if err != nil {
		return nil, err
	}
	return &Sem{sem: sem, release: release, cap: cap}, nil
}
