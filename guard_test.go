package guard

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestGuardCheckAndRelease(t *testing.T) {
	g := NewGuard()
	if err := g.Check("k1"); err != nil {
		t.Fatalf("首次 Check 应成功, 得到错误: %v", err)
	}
	if err := g.Check("k1"); err == nil {
		t.Fatal("重复 Check 应返回错误")
	}
	g.Release("k1")
	if err := g.Check("k1"); err != nil {
		t.Fatalf("Release 后 Check 应成功, 得到错误: %v", err)
	}
	g.Release("k1")
}

func TestGuardCustomErr(t *testing.T) {
	custom := errors.New("资源使用中")
	g := NewGuard(custom)
	if err := g.Check("k"); err != nil {
		t.Fatalf("首次 Check 应成功, 得到错误: %v", err)
	}
	err := g.Check("k")
	if !errors.Is(err, custom) {
		t.Fatalf("期望错误包含自定义错误 %v, 得到 %v", custom, err)
	}
	if !errors.Is(err, ErrResourceInUse) {
		t.Fatalf("期望错误包含 ErrResourceInUse, 得到 %v", err)
	}
}

func TestGuardDifferentKeys(t *testing.T) {
	g := NewGuard()
	if err := g.Check("a"); err != nil {
		t.Fatalf("Check a 失败: %v", err)
	}
	if err := g.Check("b"); err != nil {
		t.Fatalf("不同 key 不应冲突, 得到错误: %v", err)
	}
}

func TestGuardAcquireIdempotentRelease(t *testing.T) {
	g := NewGuard()

	release, err := g.Acquire("k")
	if err != nil {
		t.Fatalf("首次 Acquire 应成功, 得到错误: %v", err)
	}
	// 重复释放不应影响后来占用者（幂等）
	release()
	release()

	// 释放后应能再次占用
	if _, err := g.Acquire("k"); err != nil {
		t.Fatalf("Release 后 Acquire 应成功, 得到错误: %v", err)
	}
}

func TestGuardAcquireConflict(t *testing.T) {
	g := NewGuard()
	if _, err := g.Acquire("k"); err != nil {
		t.Fatalf("首次 Acquire 应成功, 得到错误: %v", err)
	}
	release, err := g.Acquire("k")
	if err == nil {
		release()
		t.Fatal("重复 Acquire 应返回错误")
	}
}

func TestGuardDefaultErr(t *testing.T) {
	g := NewGuard()
	if err := g.Check("k"); err != nil {
		t.Fatalf("首次 Check 应成功: %v", err)
	}
	if err := g.Check("k"); err != ErrResourceInUse {
		t.Fatalf("默认错误应为 ErrResourceInUse, 得到 %v", err)
	}
}

func TestGuardPackageLevel(t *testing.T) {
	old := defaultGuard.Load()
	defer defaultGuard.Store(old)

	if err := Check("pkg-key"); err != nil {
		t.Fatalf("包级 Check 应成功, 得到错误: %v", err)
	}
	if err := Check("pkg-key"); err == nil {
		t.Fatal("包级重复 Check 应返回错误")
	}
	Release("pkg-key")

	release, err := Acquire("pkg-key")
	if err != nil {
		t.Fatalf("包级 Acquire 应成功, 得到错误: %v", err)
	}
	release()
}

func TestGuardSetDefault(t *testing.T) {
	old := defaultGuard.Load()
	defer defaultGuard.Store(old)

	custom := errors.New("自定义占用")
	SetDefault(NewGuard(custom))
	if err := Check("k"); err != nil {
		t.Fatalf("首次 Check 应成功, 得到错误: %v", err)
	}
	if err := Check("k"); !errors.Is(err, custom) {
		t.Fatalf("期望自定义错误 %v, 得到 %v", custom, err)
	}
}

func TestGuardSetDefaultNil(t *testing.T) {
	old := defaultGuard.Load()
	defer defaultGuard.Store(old)
	SetDefault(nil) // 不应 panic，也不应替换
	if err := Check("k"); err != nil {
		t.Fatalf("SetDefault(nil) 后 Check 应仍可用, 得到错误: %v", err)
	}
}

func TestGuardConcurrent(t *testing.T) {
	g := NewGuard()
	const n = 100
	var wg sync.WaitGroup
	var success int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				release, err := g.Acquire("same")
				if err != nil {
					continue // 竞争失败，重试
				}
				atomic.AddInt32(&success, 1)
				release()
				return
			}
		}()
	}
	wg.Wait()
	if success != n {
		t.Fatalf("期望 success=%d, 得到 %d", n, success)
	}
}
