package guard

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ndsky1003/lease"
)

func newTestGuardRWMutex() *GuardRWMutex {
	return NewGuardRWMutex(time.Hour, 30*time.Second, time.Second)
}

// newShortTTLGuardRWMutex 直接构造一个短 ttl 的 GuardRWMutex，用于观察条目回收。
// 绕过 NewGuardRWMutex 对 ttl 的 1 分钟下限：该下限是为生产场景设计的，
// 单测只有用短 ttl 才能在毫秒级观察到「空闲回收」与「引用保护」的行为差异。
func newShortTTLGuardRWMutex(ttl, tick time.Duration) *GuardRWMutex {
	return &GuardRWMutex{
		TTL: ttl,
		l: lease.NewWithOptions(lease.Options[any, *_RWMutex]{
			Tick:          tick,
			RenewInterval: 0,
			Gen: func(a any) (*_RWMutex, error) {
				return &_RWMutex{RWMutex: &sync.RWMutex{}}, nil
			},
		}),
	}
}

func TestGuardRWMutex(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m := g.GetRWLock("k")
	defer m.Release()
	m.Lock()
	m.Unlock()
	m.RLock()
	m.RUnlock()
}

func TestGuardRWMutexConcurrentRead(t *testing.T) {
	const n = 100
	g := newTestGuardRWMutex()
	defer g.Close()
	var wg sync.WaitGroup
	var count int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := g.GetRWLock("counter")
			defer m.Release()
			m.RLock()
			atomic.AddInt64(&count, 1)
			m.RUnlock()
		}()
	}
	wg.Wait()
	if count != n {
		t.Fatalf("期望 count=%d, 得到 %d", n, count)
	}
}

func TestGuardRWMutexWriteBlocksRead(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m := g.GetRWLock("k")
	defer m.Release()
	m.Lock()
	acquired := make(chan struct{})
	go func() {
		m.RLock()
		close(acquired)
		m.RUnlock()
	}()
	select {
	case <-acquired:
		t.Fatal("写锁持有时读锁不应获得")
	case <-time.After(20 * time.Millisecond):
	}
	m.Unlock()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("写锁释放后读锁应获得")
	}
}

func TestGuardRWMutexReadBlocksWrite(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m := g.GetRWLock("k")
	defer m.Release()
	m.RLock()
	acquired := make(chan struct{})
	go func() {
		m.Lock()
		close(acquired)
		m.Unlock()
	}()
	select {
	case <-acquired:
		t.Fatal("读锁持有时写锁不应获得")
	case <-time.After(20 * time.Millisecond):
	}
	m.RUnlock()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("读锁释放后写锁应获得")
	}
}

func TestGuardRWMutexSameKeySameLock(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m1 := g.GetRWLock("k")
	defer m1.Release()
	m2 := g.GetRWLock("k")
	defer m2.Release()
	if m1._RWMutex != m2._RWMutex {
		t.Fatal("相同 key 应返回同一把锁")
	}
}

func TestGuardRWMutexDifferentKeys(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m1 := g.GetRWLock("a")
	defer m1.Release()
	m2 := g.GetRWLock("b")
	defer m2.Release()
	if m1._RWMutex == m2._RWMutex {
		t.Fatal("不同 key 应返回不同锁")
	}
}

func TestGuardRWMutexCloseIdempotent(t *testing.T) {
	g := newTestGuardRWMutex()
	g.Close()
	g.Close() // 幂等，不应 panic
}

func TestGuardRWMutexHandleReleaseIdempotent(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m := g.GetRWLock("k")
	m.Release()
	m.Release() // 幂等，不应 panic
}

// TestGuardRWMutexRefHeldUntilRelease 验证「一次 GetRWLock + 多对 RLock/RUnlock」时，
// 句柄在 Release 前始终持有租约引用，条目不会被空闲回收。
// 这是针对原缺陷（Unlock/RUnlock 第一次就交还引用导致条目提前回收）的回归测试：
// 改为「引用由句柄 Release 交还」，加锁/解锁本身不再触碰引用。
func TestGuardRWMutexRefHeldUntilRelease(t *testing.T) {
	g := newShortTTLGuardRWMutex(50*time.Millisecond, 10*time.Millisecond)
	defer g.Close()

	m := g.GetRWLock("k")
	// 句柄未 Release，多对 RLock/RUnlock 累计耗时超过 ttl，条目仍应存活
	for i := 0; i < 3; i++ {
		m.RLock()
		m.RUnlock()
		time.Sleep(30 * time.Millisecond)
	}
	if g.Len() != 1 {
		t.Fatalf("句柄 Release 前条目不应被回收, Len=%d", g.Len())
	}
	m.Release()
}

// TestGuardRWMutexRecycleAfterRelease 验证句柄 Release 交还引用后，
// 空闲满 ttl 时条目被正常回收（无引用泄漏）。
func TestGuardRWMutexRecycleAfterRelease(t *testing.T) {
	g := newShortTTLGuardRWMutex(50*time.Millisecond, 10*time.Millisecond)
	defer g.Close()

	m := g.GetRWLock("k")
	m.RLock()
	m.RUnlock()
	m.Release() // 交还引用

	time.Sleep(150 * time.Millisecond)
	if g.Len() != 0 {
		t.Fatalf("Release 后空闲满 ttl 应被回收, Len=%d", g.Len())
	}
}

// TestGuardRWMutexLeakWithoutRelease 验证「忘 Release」确实会导致引用泄漏（条目不回收），
// 呼应 GetRWLock 文档中「用完后必须 Release，否则条目不会被空闲回收」的约定。
func TestGuardRWMutexLeakWithoutRelease(t *testing.T) {
	g := newShortTTLGuardRWMutex(50*time.Millisecond, 10*time.Millisecond)
	defer g.Close()

	_ = g.GetRWLock("k") // 故意不 Release
	time.Sleep(150 * time.Millisecond)
	if g.Len() != 1 {
		t.Fatalf("未 Release 时条目不应被回收（引用泄漏）, Len=%d", g.Len())
	}
}
