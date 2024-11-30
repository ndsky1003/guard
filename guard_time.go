package guard

import (
	"errors"
	"sync"
	"time"
)

/*
避免客户端的防抖问题
usage

	gt := NewGuardTime(5*time.Second, errors.New("操作过多"))
	if err := gt.Handle("cc"); err != nil { 检测cc资源是否已经在极短暂的时间里使用了
		t.Error(err)
	}
*/
type guard_time struct {
	m   sync.Map
	opt *OptionGuardtime
	l   sync.Mutex
}

// 在极短的时间里，操作了这个资源
// interval 时间间隔
// err 时间间隔里的错误
func NewGuardTime(opts ...*OptionGuardtime) *guard_time {
	opt := OptionsGuardtime().
		SetInterval(5 * time.Second).
		SetClearInterval(30 * time.Second).
		SetErr(errors.New("操作过多")).
		Merge(opts...)
	g := &guard_time{
		opt: opt,
	}
	go g.auto_release(*opt.ClearInterval)
	return g
}

func (this *guard_time) Handle(key any, opts ...*OptionGuardtime) error {
	opt := OptionsGuardtime().Merge(this.opt).Merge(opts...)
	now := time.Now()
	interval := *opt.Interval
	if !this.l.TryLock() {
		return opt.Err
	}
	defer this.l.Unlock()
	old, ok := this.m.Load(key)
	if ok {
		sub := now.Sub(old.(time.Time))
		if sub < interval {
			return opt.Err
		}
	}
	this.m.Store(key, now)
	return nil
}

func (this *guard_time) auto_release(clear_interval time.Duration) {

	for {
		now := time.Now()
		this.m.Range(func(key, value any) bool {
			if now.Sub(value.(time.Time)) > clear_interval {
				this.m.Delete(key)
			}
			return true
		})
		time.Sleep(10 * time.Second)
	}
}
