package guard

import (
	"sync"
	"testing"
	"time"
)

func newTestGuardMutex() *guard_mutex {
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
