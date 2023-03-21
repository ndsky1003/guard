/*
**
该包是一个门卫用法，同时只能有一个使用者消耗一个资源，相当于数据库的行锁
*
*/
package guard

import (
	"fmt"
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
	m           sync.Map
	errTemplate string
}

func NewGuard(errTemplate string) *guard {
	return &guard{
		errTemplate: errTemplate,
	}
}

// key 资源标识符
func (this *guard) Check(key any) error {
	if _, ok := this.m.LoadOrStore(key, struct{}{}); ok {
		return fmt.Errorf(this.errTemplate, key)
	}
	return nil
}

func (this *guard) Release(key any) {
	this.m.Delete(key)
}

var g = NewGuard("frequent operation:[%s]")

func Check(key string) error {
	return g.Check(key)
}

func Release(key string) {
	g.Release(key)
}
