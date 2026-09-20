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

func NewGuardMutex(ttl time.Duration) *guard_mutex {
	if ttl < 1*time.Minute {
		ttl = time.Minute
	}
	d := &guard_mutex{
		TTL: ttl,
		l: lease.NewWithOptions(lease.Options[any, *_Mutex]{
			Tick:          30 * time.Second,
			RenewInterval: 1 * time.Second,
			Gen: func(a any) (*_Mutex, error) {
				return &_Mutex{Mutex: &sync.Mutex{}}, nil
			},
		}),
	}
	return d
}

type _Mutex struct {
	*sync.Mutex
}

type mutex struct {
	*_Mutex
	fn func()
}

func (m mutex) Lock() {
	m._Mutex.Lock()
}

func (m mutex) Unlock() {
	m._Mutex.Unlock()
	if m.fn != nil {
		m.fn()
	}
}

var gm = NewGuardMutex(time.Hour)

func GetLock(key any) *mutex {
	v, release, _ := gm.l.MustGet(key, gm.TTL)
	return &mutex{
		_Mutex: v,
		fn:     release,
	}
}

type _RWMutex struct {
	*sync.RWMutex
}

type guard_rwmutex struct {
	TTL time.Duration
	l   *lease.Lease[any, *_RWMutex]
}

func NewGuardRWMutex(ttl time.Duration) *guard_rwmutex {
	if ttl < 1*time.Minute {
		ttl = time.Minute
	}
	d := &guard_rwmutex{
		TTL: ttl,
		l: lease.NewWithOptions(lease.Options[any, *_RWMutex]{
			Tick:          30 * time.Second,
			RenewInterval: 1 * time.Second,
			Gen: func(a any) (*_RWMutex, error) {
				return &_RWMutex{RWMutex: &sync.RWMutex{}}, nil
			},
		}),
	}
	return d
}

type rwmutex struct {
	*_RWMutex
	fn func()
}

func (m rwmutex) Lock() {
	if m._RWMutex != nil {
		m._RWMutex.Lock()
	}
}

func (m rwmutex) Unlock() {
	if m._RWMutex != nil {
		m._RWMutex.Unlock()
	}
	if m.fn != nil {
		m.fn()
	}
}

func (m rwmutex) RLock() {
	if m._RWMutex != nil {
		m._RWMutex.RLock()
	}
}

func (m rwmutex) RUnlock() {
	if m._RWMutex != nil {
		m._RWMutex.RUnlock()
	}
	if m.fn != nil {
		m.fn()
	}
}

var _gm = NewGuardRWMutex(time.Hour)

func GetRWLock(key any) *rwmutex {
	v, release, _ := _gm.l.MustGet(key, _gm.TTL)
	return &rwmutex{
		_RWMutex: v,
		fn:       release,
	}
}
