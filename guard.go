/*
**
该包是一个门卫用法，同时只能有一个使用者消耗一个资源，相当于数据库的行锁
*
*/
package guard

import (
	"errors"
	"sync"

	"github.com/ndsky1003/guard/options"
)

/*useage
*	if err := guard.Check(req.ID); err != nil {
		return err
	}
	defer guard.Release(req.ID)
	主要针对资源尚未释放

	相对于guardwait，这个是直接报错，另一个是等待期释放资源
*/

type guard struct {
	m   sync.Map
	opt *options.GuardOptions
}

func NewGuard(opt *options.GuardOptions) *guard {
	if opt == nil || opt.Err == nil {
		panic("opt err must not nil")
	}
	return &guard{
		opt: opt,
	}
}

// key 资源标识符
func (this *guard) Check(key any, opts ...*options.GuardOptions) error {
	opt := options.Guard().Merge(this.opt).Merge(opts...)
	if _, ok := this.m.LoadOrStore(key, struct{}{}); ok {
		return opt.Err
	}
	return nil
}

func (this *guard) Release(key any) {
	this.m.Delete(key)
}

var g = NewGuard(options.Guard().SetErr(errors.New("frequent operation")))

func Check(key string, opts ...*options.GuardOptions) error {
	return g.Check(key, opts...)
}

func Release(key string) {
	g.Release(key)
}
