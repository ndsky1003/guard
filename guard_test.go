package guard

import (
	"errors"
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
	g := NewGuard(Options().SetErr(custom))
	if err := g.Check("k"); err != nil {
		t.Fatalf("首次 Check 应成功, 得到错误: %v", err)
	}
	if err := g.Check("k"); err != custom {
		t.Fatalf("期望自定义错误 %v, 得到 %v", custom, err)
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
