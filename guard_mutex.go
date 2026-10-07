package guard

import (
	"sync"
	"time"

	"github.com/ndsky1003/lease"
)

type guard_mutex struct {
	TTL time.Duration
	l   *lease.Lease[any, *_Mutex]
}

// NewGuardMutex 创建一个命名互斥锁管理器。
// ttl 是单把锁的空闲存活时长（低于 1 分钟会强制为 1 分钟）；
// tick 是租约到期检查粒度；renewInterval 是续期合并阈值（<=0 表示每次访问都续期）。
func NewGuardMutex(ttl, tick, renewInterval time.Duration) *guard_mutex {
	if ttl < 1*time.Minute {
		ttl = time.Minute
	}
	d := &guard_mutex{
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

// GetLock 获取 key 对应的命名互斥锁；同一 key 返回同一把底层锁，空闲超时自动回收。
func (g *guard_mutex) GetLock(key any) *mutex {
	v, release, _ := g.l.MustGet(key, g.TTL)
	return &mutex{
		_Mutex: v,
		fn:     release,
	}
}

// Close 停止内部租约时间轮并释放所有空闲条目；重复调用安全（幂等）。
func (g *guard_mutex) Close() {
	if g != nil && g.l != nil {
		g.l.Stop()
	}
}

type _Mutex struct {
	*sync.Mutex
}

type mutex struct {
	*_Mutex
	fn func()
}

func (m *mutex) Lock() {
	m._Mutex.Lock()
}

func (m *mutex) Unlock() {
	m._Mutex.Unlock()
	if m.fn != nil {
		m.fn()
	}
}
