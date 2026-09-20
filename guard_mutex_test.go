package guard

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuardMutexLock(t *testing.T) {
	m := GetLock("k")
	m.Lock()
	m.Unlock()
}

func TestGuardMutexMutualExclusion(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	var count int
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := GetLock("counter")
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

func TestGuardRWMutex(t *testing.T) {
	m := GetRWLock("k")
	m.Lock()
	m.Unlock()
	m.RLock()
	m.RUnlock()
}

func TestGuardRWMutexConcurrentRead(t *testing.T) {
	const n = 100
	var wg sync.WaitGroup
	var count int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := GetRWLock("counter")
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
	m := GetRWLock("k")
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
