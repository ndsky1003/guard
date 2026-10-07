package guard

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestGuardRWMutex() *guard_rwmutex {
	return NewGuardRWMutex(time.Hour, 30*time.Second, time.Second)
}

func TestGuardRWMutex(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m := g.GetRWLock("k")
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
	m2 := g.GetRWLock("k")
	if m1._RWMutex != m2._RWMutex {
		t.Fatal("相同 key 应返回同一把锁")
	}
}

func TestGuardRWMutexDifferentKeys(t *testing.T) {
	g := newTestGuardRWMutex()
	defer g.Close()
	m1 := g.GetRWLock("a")
	m2 := g.GetRWLock("b")
	if m1._RWMutex == m2._RWMutex {
		t.Fatal("不同 key 应返回不同锁")
	}
}

func TestGuardRWMutexCloseIdempotent(t *testing.T) {
	g := newTestGuardRWMutex()
	g.Close()
	g.Close() // 幂等，不应 panic
}
