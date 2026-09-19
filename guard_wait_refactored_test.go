package guard

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestGuardWaitGetBucket(t *testing.T) {
	gw := NewGuardWait(time.Second, time.Minute)
	defer gw.Close()

	if _, err := gw.GetBucket("", 1); err == nil {
		t.Fatal("空 key 应报错")
	}
	if _, err := gw.GetBucket("k", 0); err == nil {
		t.Fatal("cap<=0 应报错")
	}

	b, err := gw.GetBucket("k", 2)
	if err != nil {
		t.Fatalf("GetBucket 失败: %v", err)
	}
	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire 失败: %v", err)
	}
	b.Release()

	b2, _ := gw.GetBucket("k", 2)
	if b2 != b {
		t.Fatal("相同 key 应返回同一个桶")
	}
}

func TestGuardWaitLimit(t *testing.T) {
	gw := NewGuardWait(time.Second, time.Minute)
	defer gw.Close()
	b, _ := gw.GetBucket("k", 1)

	const n = 100
	var wg sync.WaitGroup
	var count int
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Acquire(context.Background()); err != nil {
				t.Errorf("Acquire 失败: %v", err)
				return
			}
			count++
			b.Release()
		}()
	}
	wg.Wait()
	if count != n {
		t.Fatalf("期望 count=%d, 得到 %d", n, count)
	}
}

func TestGuardWaitContextTimeout(t *testing.T) {
	gw := NewGuardWait(time.Second, time.Minute)
	defer gw.Close()
	b, _ := gw.GetBucket("k", 1)
	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("首次 Acquire 失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := b.Acquire(ctx); err == nil {
		t.Fatal("容量已满且超时应返回错误")
	}
	b.Release()
}
