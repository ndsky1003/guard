package guard

import (
	"sync"
	"time"

	"github.com/ndsky1003/lease"
)

type GuardMutex struct {
	TTL time.Duration
	l   *lease.Lease[any, *_Mutex]
}

// NewGuardMutex 创建一个命名互斥锁管理器。
// ttl 是单把锁的空闲存活时长（低于 1 分钟会强制为 1 分钟）；
// tick 是租约到期检查粒度；renewInterval 是续期合并阈值（<=0 表示每次访问都续期）。
func NewGuardMutex(ttl, tick, renewInterval time.Duration) *GuardMutex {
	if ttl < 1*time.Minute {
		ttl = time.Minute
	}
	d := &GuardMutex{
		TTL: ttl,
		l: lease.NewWithOptions(lease.Options[any, *_Mutex]{
			Tick:          tick,
			RenewInterval: renewInterval,
			Gen: func(a any) (*_Mutex, error) {
				return &_Mutex{Mutex: &sync.Mutex{}}, nil
			},
		}),
	}
	return d
}

// GetLock 获取 key 对应的命名互斥锁；同一 key 返回同一把底层锁。
// 返回的句柄持有一次租约引用，调用方用完后必须调用 Release 交还引用，
// 否则该 key 的锁条目不会被空闲回收（引用泄漏，锁对象永久驻留内存）。
//
// 推荐用法：
//
//	m := g.GetLock("user_id")
//	defer m.Release()
//	m.Lock()
//	defer m.Unlock()
//
// 一个句柄可安全地执行多次 Lock/Unlock；加锁/解锁本身不涉及引用管理
// （零额外开销），引用只在句柄 Release 时交还。
func (g *GuardMutex) GetLock(key any) *Mutex {
	v, release, _ := g.l.MustGet(key, g.TTL)
	return &Mutex{
		_Mutex: v,
		fn:     release,
	}
}

// Len 返回当前持有的锁条目数量（含正在使用中的）。
// 用于监控：正常值应接近「活跃 key 数」并随空闲回落；只涨不落说明有 GetLock 漏 Release。
func (g *GuardMutex) Len() int {
	if g == nil || g.l == nil {
		return 0
	}
	return g.l.Len()
}

// Close 停止内部租约时间轮并释放所有空闲条目；重复调用安全（幂等）。
func (g *GuardMutex) Close() {
	if g != nil && g.l != nil {
		g.l.Stop()
	}
}

type _Mutex struct {
	*sync.Mutex
}

type Mutex struct {
	*_Mutex
	fn func()
}

func (m *Mutex) Lock() {
	m._Mutex.Lock()
}

func (m *Mutex) Unlock() {
	m._Mutex.Unlock()
}

// Release 交还句柄持有的租约引用，允许该 key 的锁条目在空闲超时后被回收。
// 幂等且并发安全（依赖底层 release 闭包的幂等性），重复调用安全。
//
// 调用方应在拿到句柄后立即 defer Release；Release 之后不应再使用本句柄加锁——
// 否则底层锁虽仍可用，但条目已可能被回收，同 key 再次 GetLock 会拿到全新的锁，
// 互斥语义失效。
func (m *Mutex) Release() {
	if m.fn != nil {
		m.fn()
	}
}
