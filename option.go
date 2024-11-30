package guard

import "time"

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
