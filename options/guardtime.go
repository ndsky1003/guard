package options

import "time"

type GuardtimeOptions struct {
	Interval *time.Duration
	Err      error
}

func GuardTime() *GuardtimeOptions {
	return new(GuardtimeOptions)
}

func (this *GuardtimeOptions) SetInterval(i time.Duration) *GuardtimeOptions {
	if this == nil {
		return this
	}
	this.Interval = &i
	return this
}

func (this *GuardtimeOptions) SetErr(e error) *GuardtimeOptions {
	if this == nil {
		return this
	}
	this.Err = e
	return this
}

func (this *GuardtimeOptions) Merge(opts ...*GuardtimeOptions) *GuardtimeOptions {
	for _, opt := range opts {
		this.merge(opt)
	}
	return this
}

func (this *GuardtimeOptions) merge(opt *GuardtimeOptions) {
	if opt.Interval != nil {
		this.Interval = opt.Interval
	}
	if opt.Err != nil {
		this.Err = opt.Err
	}
}
