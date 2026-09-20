package guard

import (
	"errors"
	"testing"
	"time"
)

func TestGuardTimeHandle(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(50 * time.Millisecond))
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
	gt.Handle("k")
	if err := gt.Handle("k"); err != custom {
		t.Fatalf("期望自定义错误 %v, 得到 %v", custom, err)
	}
}

func TestGuardTimeDifferentKeys(t *testing.T) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(time.Second))
	if err := gt.Handle("a"); err != nil {
		t.Fatalf("Handle a 失败: %v", err)
	}
	if err := gt.Handle("b"); err != nil {
		t.Fatalf("不同 key 不应冲突, 得到错误: %v", err)
	}
}
