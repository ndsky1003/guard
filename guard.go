/*
**
该包是一个门卫用法，同时只能有一个使用者消耗一个资源，相当于数据库的行锁
*
*/
package guard

import (
	"errors"
	"sync"
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
	sync.Map
	opt *Option
}

func NewGuard(opts ...*Option) *guard {
	opt := Options().
		SetErr(errors.New("Frequent handling")).
		Merge(opts...)
	return &guard{
		opt: opt,
	}
}

// key 资源标识符
func (this *guard) Check(key any, opts ...*Option) error {
	opt := Options().Merge(this.opt).Merge(opts...)
	if _, ok := this.LoadOrStore(key, struct{}{}); ok {
		return opt.Err
	}
	return nil
}

func (this *guard) Release(key any) {
	this.Delete(key)
}

var g = NewGuard()

func Check(key string, opts ...*Option) error {
	return g.Check(key, opts...)
}

func Release(key string) {
	g.Release(key)
}

// ========================option========================
type Option struct {
	Err error
}

func Options() *Option {
	return &Option{}
}

func (this *Option) SetErr(e error) *Option {
	if this == nil {
		return this
	}
	this.Err = e
	return this
}

func (this *Option) Merge(opts ...*Option) *Option {
	for _, opt := range opts {
		this.merge(opt)
	}
	return this
}

func (this *Option) merge(opt *Option) {
	if opt == nil {
		return
	}
	if opt.Err != nil {
		this.Err = opt.Err
	}
}
