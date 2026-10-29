package guard

import (
	"sync"
	"testing"
	"time"

	"github.com/ndsky1003/lease"
)

func newTestGuardMutex() *GuardMutex {
	return NewGuardMutex(time.Hour, 30*time.Second, time.Second)
}

func TestGuardMutexLock(t *testing.T) {
	g := newTestGuardMutex()
	defer g.Close()
	m := g.GetLock("k")
	defer m.Release()
	m.Lock()
	m.Unlock()
}

func TestGuardMutexMutualExclusion(t *testing.T) {
	const n = 100
	g := newTestGuardMutex()
	defer g.Close()
	var wg sync.WaitGroup
	var count int
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := g.GetLock("counter")
			defer m.Release()
			m.Lock()
			count++
			m.Unlock()
		}()
	}
	wg.Wait()
	if count != n {
		t.Fatalf("期望 count=%d, 得到 %d", n, count)
	}
}

func TestGuardMutexTTLMin(t *testing.T) {
	g := NewGuardMutex(time.Millisecond, 30*time.Second, time.Second)
	defer g.Close()
	if g.TTL != time.Minute {
		t.Fatalf("ttl 低于 1 分钟应强制为 1 分钟, 得到 %v", g.TTL)
	}
}

func TestGuardMutexSameKeySameLock(t *testing.T) {
	g := newTestGuardMutex()
	defer g.Close()
	m1 := g.GetLock("k")
	defer m1.Release()
	m2 := g.GetLock("k")
	defer m2.Release()
	if m1._Mutex != m2._Mutex {
		t.Fatal("相同 key 应返回同一把锁")
	}
}

func TestGuardMutexDifferentKeys(t *testing.T) {
	g := newTestGuardMutex()
	defer g.Close()
	m1 := g.GetLock("a")
	defer m1.Release()
	m2 := g.GetLock("b")
	defer m2.Release()
	if m1._Mutex == m2._Mutex {
		t.Fatal("不同 key 应返回不同锁")
	}
}

func TestGuardMutexCloseIdempotent(t *testing.T) {
	g := newTestGuardMutex()
	g.Close()
	g.Close() // 幂等，不应 panic
}

// newShortTTLGuardMutex 直接构造一个短 ttl 的 GuardMutex，用于观察条目回收。
// 绕过 NewGuardMutex 对 ttl 的 1 分钟下限（该下限是为生产场景设计的）。
func newShortTTLGuardMutex(ttl, tick time.Duration) *GuardMutex {
	return &GuardMutex{
		TTL: ttl,
		l: lease.NewWithOptions(lease.Options[any, *_Mutex]{
			Tick:          tick,
			RenewInterval: 0,
			Gen: func(a any) (*_Mutex, error) {
				return &_Mutex{Mutex: &sync.Mutex{}}, nil
			},
		}),
	}
}

func TestGuardMutexReleaseIdempotent(t *testing.T) {
	g := newTestGuardMutex()
	defer g.Close()
	m := g.GetLock("k")
	m.Release()
	m.Release() // 幂等，不应 panic
}

// TestGuardMutexRefHeldUntilRelease 验证句柄在 Release 前始终持有租约引用，
// 多次 Lock/Unlock 后条目仍不会被空闲回收（引用由句柄 Release 交还，而非 Unlock）。
func TestGuardMutexRefHeldUntilRelease(t *testing.T) {
	g := newShortTTLGuardMutex(50*time.Millisecond, 10*time.Millisecond)
	defer g.Close()

	m := g.GetLock("k")
	for i := 0; i < 3; i++ {
		m.Lock()
		m.Unlock()
		time.Sleep(30 * time.Millisecond)
	}
	if g.Len() != 1 {
		t.Fatalf("句柄 Release 前条目不应被回收, Len=%d", g.Len())
	}
	m.Release()
}

// TestGuardMutexRecycleAfterRelease 验证句柄 Release 交还引用后，
// 空闲满 ttl 时条目被正常回收（无引用泄漏）。
func TestGuardMutexRecycleAfterRelease(t *testing.T) {
	g := newShortTTLGuardMutex(50*time.Millisecond, 10*time.Millisecond)
	defer g.Close()

	m := g.GetLock("k")
	m.Lock()
	m.Unlock()
	m.Release()

	time.Sleep(150 * time.Millisecond)
	if g.Len() != 0 {
		t.Fatalf("Release 后空闲满 ttl 应被回收, Len=%d", g.Len())
	}
}
