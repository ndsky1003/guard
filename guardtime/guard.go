package guardtime

import (
	"sync"
	"time"

	"github.com/ndsky1003/guard/options"
)

/*
避免客户端的防抖问题
usage

	gt := NewGuardTime(5*time.Second, errors.New("操作过多"))
	if err := gt.Handle("cc"); err != nil { 检测cc资源是否已经在极端的时间里使用了
		t.Error(err)
	}
*/
type guard_time struct {
	interval    time.Duration
	m           sync.Map
	errTemplate error
}

// 在极短的时间里，操作了这个资源
// interval 时间间隔
// err 时间间隔里的错误
func NewGuardTime(interval time.Duration, err error) *guard_time {
	g := &guard_time{
		interval:    interval,
		errTemplate: err,
	}
	go g.auto_release()
	return g
}

func (this *guard_time) Handle(key any, opts ...*options.GuardtimeOptions) error {
	now := time.Now()
	interval := this.interval
	opt := options.GuardTime().Merge(opts...)
	if opt.Interval != nil {
		interval = *opt.Interval
	}
	if old, ok := this.m.LoadOrStore(key, now); ok {
		sub := now.Sub(old.(time.Time))
		if sub < interval {
			if opt.Err != nil {
				return opt.Err
			}
			return this.errTemplate
		}
	}
	this.m.Store(key, now)
	return nil
}

func (this *guard_time) auto_release() {
	for {
		now := time.Now()
		this.m.Range(func(key, value any) bool {
			if now.Sub(value.(time.Time)) >= this.interval {
				this.m.Delete(key)
			}
			return true
		})
		time.Sleep(1 * time.Second)
	}
}
