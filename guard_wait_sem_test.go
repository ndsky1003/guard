package guard

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuardWaitGetSem(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()

	if _, err := gw.GetSem("", 1); err == nil {
		t.Fatal("空 key 应报错")
	}
	if _, err := gw.GetSem("k", 0); err == nil {
		t.Fatal("cap<=0 应报错")
	}

	b, err := gw.GetSem("k", 2)
	if err != nil {
		t.Fatalf("GetSem 失败: %v", err)
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}
	b.ReleaseTicket(1)
	b.Release()

	b2, _ := gw.GetSem("k", 2)
	if b2.sem != b.sem {
		t.Fatal("相同 key 应返回同一个信号量")
	}
}

func TestGuardWaitLimit(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	b, _ := gw.GetSem("k", 1)
	defer b.Release()

	const n = 100
	var wg sync.WaitGroup
	var count int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Acquire(context.Background(), 1); err != nil {
				t.Errorf("Acquire 失败: %v", err)
				return
			}
			atomic.AddInt32(&count, 1)
			b.ReleaseTicket(1)
		}()
	}
	wg.Wait()
	if count != n {
		t.Fatalf("期望 count=%d, 得到 %d", n, count)
	}
}

func TestGuardWaitContextTimeout(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	b, _ := gw.GetSem("k", 1)
	defer b.Release()

	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("首次 Acquire 失败: %v", err)
	}
	defer b.ReleaseTicket(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := b.Acquire(ctx, 1); err == nil {
		t.Fatal("容量已满且超时应返回错误")
	}
}

// TestGuardWaitBucketNotEvictedWhileAcquired 验证许可占用期间信号量不会被空闲回收，
// 释放并超过 ttl 后才被回收（修复限流失效的核心场景）。
func TestGuardWaitBucketNotEvictedWhileAcquired(t *testing.T) {
	gw := NewGuardWaitSem(10*time.Millisecond, 50*time.Millisecond)
	defer gw.Close()

	b, err := gw.GetSem("k", 1)
	if err != nil {
		t.Fatalf("GetSem 失败: %v", err)
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}

	// 占用时间远超 ttl，信号量不应被回收。
	time.Sleep(200 * time.Millisecond)
	if !gw.l.Has("k:1") {
		t.Fatal("占用期间信号量被回收了")
	}

	b.ReleaseTicket(1)
	b.Release()

	// 释放后空闲超过 ttl，信号量应被回收。
	time.Sleep(200 * time.Millisecond)
	if gw.l.Has("k:1") {
		t.Fatal("释放后信号量未回收")
	}
}

func TestGuardWaitSemWeighted(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	sem, _ := gw.GetSem("k", 10)
	defer sem.Release()

	if err := sem.Acquire(context.Background(), 3); err != nil {
		t.Fatalf("Acquire(3) 失败: %v", err)
	}
	if err := sem.Acquire(context.Background(), 7); err != nil {
		t.Fatalf("Acquire(7) 失败: %v", err)
	}

	// 已满，再获取应超时
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := sem.Acquire(ctx, 1); err == nil {
		t.Fatal("容量已满应超时")
	}

	sem.ReleaseTicket(3)
	if err := sem.Acquire(context.Background(), 3); err != nil {
		t.Fatalf("释放后 Acquire(3) 失败: %v", err)
	}
	sem.ReleaseTicket(10)
}

func TestGuardWaitSemDifferentCap(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	s1, _ := gw.GetSem("k", 1)
	s2, _ := gw.GetSem("k", 2)
	defer s1.Release()
	defer s2.Release()

	if s1.sem == s2.sem {
		t.Fatal("相同 key 不同 cap 不应复用同一信号量")
	}
}

func TestGuardWaitSemInvalidN(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	sem, _ := gw.GetSem("k", 2)
	defer sem.Release()

	if err := sem.Acquire(context.Background(), 0); err == nil {
		t.Fatal("n=0 应报错")
	}
	if err := sem.Acquire(context.Background(), -1); err == nil {
		t.Fatal("n<0 应报错")
	}
	if err := sem.Acquire(context.Background(), 3); err == nil {
		t.Fatal("n>cap 应报错")
	}
}

func TestGuardWaitSemReleaseTicketInvalid(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	sem, _ := gw.GetSem("k", 2)
	defer sem.Release()

	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}
	sem.ReleaseTicket(1)

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("ReleaseTicket(0) 应 panic")
			}
		}()
		sem.ReleaseTicket(0)
	}()
}

func TestGuardWaitSemAcquireFailNoTicket(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	sem, _ := gw.GetSem("k", 1)
	defer sem.Release()

	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("首次 Acquire 失败: %v", err)
	}

	// 已满，超时失败，不应占用许可
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := sem.Acquire(ctx, 1); err == nil {
		t.Fatal("应超时")
	}

	// 释放后能正常获取
	sem.ReleaseTicket(1)
	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("释放后 Acquire 失败: %v", err)
	}
	sem.ReleaseTicket(1)
}

func TestGuardWaitSemCloseIdempotent(t *testing.T) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	gw.Close()
	gw.Close() // 幂等，不应 panic
}
