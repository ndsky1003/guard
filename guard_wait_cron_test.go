package guard

import (
	"sync"
	"testing"
	"time"
)

func TestGuardWaitCondTicket(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	b := g.GetBucket("k", 2)

	if !b.TryGotTicket() {
		t.Fatal("TryGotTicket 应成功")
	}
	b.ReleaseTicket()

	b.GotTicket()
	b.ReleaseTicket()
}

func TestGuardWaitCondTryExhaust(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	b := g.GetBucket("k", 1)

	if !b.TryGotTicket() {
		t.Fatal("首次 TryGotTicket 应成功")
	}
	if b.TryGotTicket() {
		t.Fatal("容量已满 TryGotTicket 应失败")
	}
	b.ReleaseTicket()
	if !b.TryGotTicket() {
		t.Fatal("释放后 TryGotTicket 应成功")
	}
	b.ReleaseTicket()
}

func TestGuardWaitCondMutualExclusion(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	b := g.GetBucket("k", 1)

	const n = 100
	var wg sync.WaitGroup
	var count int
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.GotTicket()
			count++
			b.ReleaseTicket()
		}()
	}
	wg.Wait()
	if count != n {
		t.Fatalf("期望 count=%d, 得到 %d", n, count)
	}
}
