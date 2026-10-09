package guard

import (
	"sync"
	"testing"
	"time"
)

func newTestGuardMutex() *GuardMutex {
	return NewGuardMutex(time.Hour, 30*time.Second, time.Second)
}

func TestGuardMutexLock(t *testing.T) {
	g := newTestGuardMutex()
	defer g.Close()
	m := g.GetLock("k")
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
	m2 := g.GetLock("k")
	if m1._Mutex != m2._Mutex {
		t.Fatal("相同 key 应返回同一把锁")
	}
}

func TestGuardMutexDifferentKeys(t *testing.T) {
	g := newTestGuardMutex()
	defer g.Close()
	m1 := g.GetLock("a")
	m2 := g.GetLock("b")
	if m1._Mutex == m2._Mutex {
		t.Fatal("不同 key 应返回不同锁")
	}
}

func TestGuardMutexCloseIdempotent(t *testing.T) {
	g := newTestGuardMutex()
	g.Close()
	g.Close() // 幂等，不应 panic
}
