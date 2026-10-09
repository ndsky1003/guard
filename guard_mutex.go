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

// GetLock 获取 key 对应的命名互斥锁；同一 key 返回同一把底层锁，空闲超时自动回收。
func (g *GuardMutex) GetLock(key any) *Mutex {
	v, release, _ := g.l.MustGet(key, g.TTL)
	return &Mutex{
		_Mutex: v,
		fn:     release,
	}
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
	if m.fn != nil {
		m.fn()
	}
}
