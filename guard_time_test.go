package guard

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuardTimeHandle(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(50 * time.Millisecond))
	defer gt.Close()
	if err := gt.Handle("k"); err != nil {
		t.Fatalf("首次 Handle 应成功, 得到错误: %v", err)
	}
	if err := gt.Handle("k"); err == nil {
		t.Fatal("间隔内重复 Handle 应报错")
	}
	time.Sleep(60 * time.Millisecond)
	if err := gt.Handle("k"); err != nil {
		t.Fatalf("超过间隔后 Handle 应成功, 得到错误: %v", err)
	}
}

func TestGuardTimeCustomErr(t *testing.T) {
	custom := errors.New("操作频繁")
	gt := NewGuardTime(OptionsGuardtime().
		SetInterval(50 * time.Millisecond).
		SetErr(custom))
	defer gt.Close()
	gt.Handle("k")
	if err := gt.Handle("k"); err != custom {
		t.Fatalf("期望自定义错误 %v, 得到 %v", custom, err)
	}
}

func TestGuardTimeDifferentKeys(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(time.Second))
	defer gt.Close()
	if err := gt.Handle("a"); err != nil {
		t.Fatalf("Handle a 失败: %v", err)
	}
	if err := gt.Handle("b"); err != nil {
		t.Fatalf("不同 key 不应冲突, 得到错误: %v", err)
	}
}

func TestGuardTimeClose(t *testing.T) {
	gt := NewGuardTime()
	gt.Close()
	gt.Close() // 重复调用应安全（幂等），不应 panic
}

func TestGuardTimeDefaultErr(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(50 * time.Millisecond))
	defer gt.Close()
	gt.Handle("k")
	if err := gt.Handle("k"); !errors.Is(err, ErrTooManyOperations) {
		t.Fatalf("期望默认错误 ErrTooManyOperations, 得到 %v", err)
	}
}

func TestGuardTimePerKeyInterval(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(time.Second))
	defer gt.Close()

	// 通过 OptionGuardtimeHandle 为单次调用覆盖更短的间隔
	if err := gt.Handle("k", OptionsGuardtimeHandle().SetInterval(20*time.Millisecond)); err != nil {
		t.Fatalf("首次 Handle 失败: %v", err)
	}
	if err := gt.Handle("k", OptionsGuardtimeHandle().SetInterval(20*time.Millisecond)); err == nil {
		t.Fatal("20ms 内重复 Handle 应报错")
	}
	time.Sleep(30 * time.Millisecond)
	if err := gt.Handle("k", OptionsGuardtimeHandle().SetInterval(20*time.Millisecond)); err != nil {
		t.Fatalf("超过间隔后 Handle 应成功: %v", err)
	}
}

func TestGuardTimePerKeyErr(t *testing.T) {
	custom := errors.New("自定义频繁")
	gt := NewGuardTime(OptionsGuardtime().SetInterval(50 * time.Millisecond))
	defer gt.Close()
	gt.Handle("k")
	if err := gt.Handle("k", OptionsGuardtimeHandle().SetErr(custom)); err != custom {
		t.Fatalf("期望按 key 覆盖的错误 %v, 得到 %v", custom, err)
	}
}

func TestGuardTimeHandleNilOpts(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(50 * time.Millisecond))
	defer gt.Close()
	if err := gt.Handle("k", nil); err != nil {
		t.Fatalf("nil opts 应正常处理: %v", err)
	}
}

func TestGuardTimeMerge(t *testing.T) {
	opt := OptionsGuardtime().
		SetInterval(10 * time.Millisecond).
		SetErr(errors.New("默认错误"))
	// Merge 仅覆盖非零值
	opt.Merge(OptionsGuardtime().SetInterval(20 * time.Millisecond))
	if *opt.Interval != 20*time.Millisecond {
		t.Fatalf("Merge 后 Interval 应为 20ms, 得到 %v", *opt.Interval)
	}
	if opt.Err == nil {
		t.Fatal("Merge 不应覆盖未设置的 Err")
	}
}

// TestGuardTimeConcurrentOnlyOnePasses 验证防抖的并发正确性：
// 100 个 goroutine 对同一 key 同时发车，间隔内只能有一个被放行。
// 该测试针对曾经的并发穿透缺陷（Get 与 Set 分离导致的竞态）。
func TestGuardTimeConcurrentOnlyOnePasses(t *testing.T) {
	const (
		waves    = 20
		perWave  = 100
		interval = 50 * time.Millisecond
	)
	gt := NewGuardTime(OptionsGuardtime().SetInterval(interval))
	defer gt.Close()

	for w := 0; w < waves; w++ {
		var passed int64
		var wg sync.WaitGroup
		wg.Add(perWave)
		for i := 0; i < perWave; i++ {
			go func() {
				defer wg.Done()
				if gt.Handle("same-key") == nil {
					atomic.AddInt64(&passed, 1)
				}
			}()
		}
		wg.Wait()

		if passed != 1 {
			t.Fatalf("第 %d 波：期望恰好 1 个通过，实际 %d 个", w+1, passed)
		}

		// 等待超过防抖间隔，让下一波重新进入新窗口
		time.Sleep(interval + 20*time.Millisecond)
	}
}

// TestGuardTimeNonPositiveInterval 验证 interval<=0 时视为不防抖：
// 每次 Handle 都放行，且不写入租约（避免 ttl<=0 条目永久驻留内存）。
func TestGuardTimeNonPositiveInterval(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(0))
	defer gt.Close()

	if err := gt.Handle("k"); err != nil {
		t.Fatalf("interval=0 应放行, 得到 %v", err)
	}
	if err := gt.Handle("k"); err != nil {
		t.Fatalf("interval=0 应始终放行, 得到 %v", err)
	}
	if gt.Len() != 0 {
		t.Fatalf("interval<=0 不应写入租约, Len=%d", gt.Len())
	}
}
