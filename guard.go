// Package guard 提供了一组「门卫」并发控制原语：
//   - Guard：行锁语义，同一时刻仅一个使用者占用同一资源，冲突直接报错；
//   - GuardTime：防抖，资源在极短时间内被重复操作时报错；
//   - GuardWait：限流，按 key 分组排队等待获取许可；
//   - GuardMutex / GuardRWMutex：带 TTL 的命名互斥锁与读写锁。
package guard

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Guard 行锁门卫：同一时刻仅允许一个使用者占用同一 key，
// 冲突时直接返回错误（而非等待），相当于数据库的行锁。
//
// 用法：
//
//	if err := guard.Check(req.ID); err != nil {
//		return err
//	}
//	defer guard.Release(req.ID)
//
// 主要针对资源尚未释放的场景。
// 相对于 GuardWait，Guard 在冲突时直接报错，而 GuardWait 是等待资源释放。
//
// 注意：Check/Release 是无所有权校验的轻量锁，调用方必须保证
// 「谁 Check 成功谁负责 Release，且只 Release 一次」，否则可能误删
// 后来占用者的锁（ABA 问题）。更安全的用法是 Acquire 返回的 release 函数。
type Guard struct {
	m   sync.Map
	err error
}

// NewGuard 创建一个行锁门卫。
// 可传入自定义错误作为冲突时返回的错误；不传则默认使用 ErrResourceInUse。
// 传入错误时，会与 ErrResourceInUse 通过 errors.Join 合并，
// 使 errors.Is(err, ErrResourceInUse) 始终成立，同时保留自定义错误的可判断性。
// 注意：合并后 err.Error() 为多行文本，若需展示给用户请在应用层自行映射。
func NewGuard(errs ...error) *Guard {
	err := ErrResourceInUse
	if len(errs) > 0 {
		err = errors.Join(append([]error{ErrResourceInUse}, errs...)...)
	}
	return &Guard{err: err}
}

// Check 尝试占用 key；若 key 已被占用则返回错误，否则返回 nil 表示占用成功。
// 占用成功后调用方需负责调用 Release 释放。
func (this *Guard) Check(key string) error {
	if _, ok := this.m.LoadOrStore(key, struct{}{}); ok {
		return this.err
	}
	return nil
}

// Acquire 尝试占用 key，成功时返回一个与本次占用绑定的释放函数。
// 释放函数保证幂等（内部 sync.Once），重复调用不会误删后来占用者的锁，
// 是比 Check/Release 更安全的用法。
func (this *Guard) Acquire(key string) (release func(), err error) {
	if _, ok := this.m.LoadOrStore(key, struct{}{}); ok {
		return nil, this.err
	}
	var once sync.Once
	return func() {
		once.Do(func() { this.m.Delete(key) })
	}, nil
}

// Release 释放 key 占用的锁。
// 该方法不做所有权校验，调用方必须保证只释放自己 Check 成功的 key，
// 且每个成功的 Check 只对应一次 Release，否则可能误删后来占用者的锁。
func (this *Guard) Release(key string) {
	this.m.Delete(key)
}

var defaultGuard atomic.Pointer[Guard]

func init() {
	defaultGuard.Store(NewGuard())
}

// SetDefault 替换包级 Check/Acquire/Release 使用的默认实例，
// 便于测试注入或全局自定义错误。建议在程序启动阶段调用；
// 若在运行中替换，不保证同一时刻的调用都使用同一实例。
func SetDefault(g *Guard) {
	if g == nil {
		return
	}
	defaultGuard.Store(g)
}

// Check 使用默认实例的 Check，见 Guard.Check。
func Check(key string) error {
	return defaultGuard.Load().Check(key)
}

// Acquire 使用默认实例的 Acquire，见 Guard.Acquire。
func Acquire(key string) (func(), error) {
	return defaultGuard.Load().Acquire(key)
}

// Release 使用默认实例的 Release，见 Guard.Release。
func Release(key string) {
	defaultGuard.Load().Release(key)
}
