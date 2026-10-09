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

// GetRWLock 获取 key 对应的命名读写锁；同一 key 返回同一把底层锁，空闲超时自动回收。
func (g *GuardRWMutex) GetRWLock(key any) *RWMutex {
	v, release, _ := g.l.MustGet(key, g.TTL)
	return &RWMutex{
		_RWMutex: v,
		fn:       release,
	}
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
	if m.fn != nil {
		m.fn()
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
	if m.fn != nil {
		m.fn()
	}
}
