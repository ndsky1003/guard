package guard

import (
	"errors"
	"time"

	"github.com/ndsky1003/lease"
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
	opt *OptionGuardtime
	l   *lease.Lease[any, time.Time]
}

// 在极短的时间里，操作了这个资源
// interval 时间间隔
// err 时间间隔里的错误
func NewGuardTime(opts ...*OptionGuardtime) *guard_time {
	opt := OptionsGuardtime().
		SetInterval(5 * time.Second).
		SetClearInterval(10 * time.Minute).
		SetErr(errors.New("Too many operations")).
		Merge(opts...)
	g := &guard_time{
		opt: opt,
	}
	g.l = lease.NewWithOptions(lease.Options[any, time.Time]{Tick: 10 * time.Minute, RenewInterval: 1 * time.Second})

	return g
}

func (this *guard_time) Handle(key any, opts ...*OptionGuardtime) error {
	opt := OptionsGuardtime().Merge(this.opt).Merge(opts...)
	now := time.Now()
	interval := *opt.Interval

	old, release, ok := this.l.Get(key)
	if ok {
		defer release()
		if now.Sub(old) < interval {
			return opt.Err
		}
	}
	this.l.Set(key, time.Now(), 2*interval)
	return nil
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
