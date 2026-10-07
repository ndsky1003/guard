package guard

import (
	"sync"
	"testing"
	"time"
)

func TestGuardWaitCondTicket(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	b, err := g.GetBucket("k", 2)
	if err != nil {
		t.Fatalf("GetBucket 失败: %v", err)
	}
	defer b.Release()

	if !b.TryGotTicket() {
		t.Fatal("TryGotTicket 应成功")
	}
	b.ReleaseTicket()

	b.GotTicket()
	b.ReleaseTicket()
}

func TestGuardWaitCondTryExhaust(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	b, err := g.GetBucket("k", 1)
	if err != nil {
		t.Fatalf("GetBucket 失败: %v", err)
	}
	defer b.Release()

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
	defer g.Close()
	b, err := g.GetBucket("k", 1)
	if err != nil {
		t.Fatalf("GetBucket 失败: %v", err)
	}
	defer b.Release()

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

func TestGuardWaitCondDifferentCap(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	b1, _ := g.GetBucket("k", 1)
	b2, _ := g.GetBucket("k", 2)
	defer b1.Release()
	defer b2.Release()

	if b1.b == b2.b {
		t.Fatal("相同 key 不同 cap 不应复用同一桶")
	}
}

func TestGuardWaitCondInvalidArgs(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	if _, err := g.GetBucket("", 1); err == nil {
		t.Fatal("空 key 应报错")
	}
	if _, err := g.GetBucket("k", 0); err == nil {
		t.Fatal("cap<=0 应报错")
	}
}

func TestGuardWaitCondReleaseTicketOverflow(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	b, _ := g.GetBucket("k", 1)
	defer b.Release()

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("多释放应 panic")
			}
		}()
		b.ReleaseTicket() // 未获取票就释放，应 panic
	}()
}

func TestGuardWaitCondCloseIdempotent(t *testing.T) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	g.Close()
	g.Close() // 幂等，不应 panic
}
