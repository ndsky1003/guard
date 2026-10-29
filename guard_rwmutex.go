package guard

import (
	"sync"
	"time"

	"github.com/ndsky1003/lease"
)

type _RWMutex struct {
	*sync.RWMutex
}

type GuardRWMutex struct {
	TTL time.Duration
	l   *lease.Lease[any, *_RWMutex]
}

// NewGuardRWMutex 创建一个命名读写锁管理器。
// ttl 是单把锁的空闲存活时长（低于 1 分钟会强制为 1 分钟）；
// tick 是租约到期检查粒度；renewInterval 是续期合并阈值（<=0 表示每次访问都续期）。
func NewGuardRWMutex(ttl, tick, renewInterval time.Duration) *GuardRWMutex {
	if ttl < 1*time.Minute {
		ttl = time.Minute
	}
	d := &GuardRWMutex{
		TTL: ttl,
		l: lease.NewWithOptions(lease.Options[any, *_RWMutex]{
			Tick:          tick,
			RenewInterval: renewInterval,
			Gen: func(a any) (*_RWMutex, error) {
				return &_RWMutex{RWMutex: &sync.RWMutex{}}, nil
			},
		}),
	}
	return d
}

// GetRWLock 获取 key 对应的命名读写锁；同一 key 返回同一把底层锁。
// 返回的句柄持有一次租约引用，调用方用完后必须调用 Release 交还引用，
// 否则该 key 的锁条目不会被空闲回收（引用泄漏，锁对象永久驻留内存）。
//
// 推荐用法：
//
//	m := g.GetRWLock("user_id")
//	defer m.Release()
//	m.Lock()
//	defer m.Unlock()
//
// 一个句柄可安全地执行多对 Lock/Unlock、RLock/RUnlock；加锁/解锁本身不涉及
// 引用管理（零额外开销），引用只在句柄 Release 时交还。
func (g *GuardRWMutex) GetRWLock(key any) *RWMutex {
	v, release, _ := g.l.MustGet(key, g.TTL)
	return &RWMutex{
		_RWMutex: v,
		fn:       release,
	}
}

// Len 返回当前持有的读写锁条目数量（含正在使用中的）。用于监控与测试。
func (g *GuardRWMutex) Len() int {
	if g == nil || g.l == nil {
		return 0
	}
	return g.l.Len()
}

// Close 停止内部租约时间轮并释放所有空闲条目；重复调用安全（幂等）。
func (g *GuardRWMutex) Close() {
	if g != nil && g.l != nil {
		g.l.Stop()
	}
}

type RWMutex struct {
	*_RWMutex
	fn func()
}

func (m *RWMutex) Lock() {
	if m._RWMutex != nil {
		m._RWMutex.Lock()
	}
}

func (m *RWMutex) Unlock() {
	if m._RWMutex != nil {
		m._RWMutex.Unlock()
	}
}

func (m *RWMutex) RLock() {
	if m._RWMutex != nil {
		m._RWMutex.RLock()
	}
}

func (m *RWMutex) RUnlock() {
	if m._RWMutex != nil {
		m._RWMutex.RUnlock()
	}
}

// Release 交还句柄持有的租约引用，允许该 key 的锁条目在空闲超时后被回收。
// 幂等且并发安全（依赖底层 release 闭包的幂等性），重复调用安全。
//
// 调用方应在拿到句柄后立即 defer Release；Release 之后不应再使用本句柄加锁——
// 否则底层锁虽仍可用，但条目已可能被回收，同 key 再次 GetRWLock 会拿到全新的锁，
// 互斥语义失效。
func (m *RWMutex) Release() {
	if m.fn != nil {
		m.fn()
	}
}
