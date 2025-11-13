package guard

import (
	"errors"
	"sync"
	"time"
)

/*
避免客户端的防抖问题
usage

	gt := NewGuardTime(5*time.Second)
	if err := gt.Handle("cc"); err != nil { 检测cc资源是否已经在极短暂的时间里使用了
		t.Error(err)
	}
*/
type guard_time struct {
	sync.Map
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
	go g.auto_release()
	return g
}

func (this *guard_time) Handle(key any, opts ...*OptionGuardtime) error {
	opt := OptionsGuardtime().Merge(this.opt).Merge(opts...)
	now := time.Now()
	interval := *opt.Interval
	this.l.Lock()
	defer this.l.Unlock()
	if old, ok := this.Load(key); ok && now.Sub(old.(time.Time)) < interval {
		return opt.Err
	}
	this.Store(key, now)
	return nil
}

func (this *guard_time) auto_release() {
	opt := this.opt
	for {
		now := time.Now()
		this.Range(func(key, value any) bool {
			if now.Sub(value.(time.Time)) > *opt.Interval {
				this.Delete(key)
			}
			return true
		})
		time.Sleep(*opt.ClearInterval * time.Second)
	}
}

type OptionGuardtime struct {
	Interval      *time.Duration //eg:5s 5s内操作多次就会报错
	ClearInterval *time.Duration //eg:30s 一个key30s内没被消费就清理掉
	Err           error
}

func OptionsGuardtime() *OptionGuardtime {
	return new(OptionGuardtime)
}

func (this *OptionGuardtime) SetInterval(i time.Duration) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.Interval = &i
	return this
}

func (this *OptionGuardtime) SetClearInterval(i time.Duration) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.ClearInterval = &i
	return this
}

func (this *OptionGuardtime) SetErr(e error) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.Err = e
	return this
}

func (this *OptionGuardtime) Merge(opts ...*OptionGuardtime) *OptionGuardtime {
	for _, opt := range opts {
		this.merge(opt)
	}
	return this
}

func (this *OptionGuardtime) merge(opt *OptionGuardtime) {
	if opt == nil {
		return
	}
	if opt.Interval != nil {
		this.Interval = opt.Interval
	}

	if opt.ClearInterval != nil {
		this.ClearInterval = opt.ClearInterval
	}

	if opt.Err != nil {
		this.Err = opt.Err
	}
}
