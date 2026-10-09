package guard

import (
	"time"

	"github.com/ndsky1003/lease"
)

type GuardTime struct {
	opt *OptionGuardtime
	l   *lease.Lease[any, time.Time]
}

// NewGuardTime 创建一个防抖门卫，用于避免客户端在极短时间内重复操作
// （如重复提交、重复发送验证码）。
// 同一 key 在 Interval 内被重复 Handle 时返回错误，超过间隔后恢复。
//
// 用法：
//
//	gt := NewGuardTime(OptionsGuardtime().SetInterval(5 * time.Second))
//	if err := gt.Handle("user_id"); err != nil {
//		// 5 秒内重复请求会走到这里
//	}
func NewGuardTime(opts ...*OptionGuardtime) *GuardTime {
	opt := OptionsGuardtime().
		SetInterval(5 * time.Second).
		SetTick(10 * time.Second).
		SetRenewInterval(1 * time.Second).
		SetErr(ErrTooManyOperations).
		Merge(opts...)
	g := &GuardTime{
		opt: opt,
	}
	g.l = lease.NewWithOptions(lease.Options[any, time.Time]{
		Tick:          *opt.Tick,
		RenewInterval: *opt.RenewInterval,
	})

	return g
}

// Handle 处理一次对 key 的访问。
// 若 key 在 Interval 内已被访问过，则返回配置的错误；否则记录本次访问并返回 nil。
// 可通过 opts 按 key 覆盖实例默认的防抖间隔和错误（不同 key 可用不同间隔）。
func (this *GuardTime) Handle(key any, opts ...*OptionGuardtimeHandle) error {
	interval := *this.opt.Interval
	err := this.opt.Err
	for _, h := range opts {
		if h == nil {
			continue
		}
		if h.Interval != nil {
			interval = *h.Interval
		}
		if h.Err != nil {
			err = h.Err
		}
	}
	now := time.Now()
	old, release, ok := this.l.Get(key)
	if ok {
		defer release()
		if now.Sub(old) < interval {
			return err
		}
	}
	this.l.Set(key, now, interval)
	return nil
}

// Close 停止内部租约时间轮并释放所有条目；重复调用安全（幂等）。
func (this *GuardTime) Close() {
	if this != nil && this.l != nil {
		this.l.Stop()
	}
}

// OptionGuardtime 是防抖门卫的实例级配置项，在 NewGuardTime 时确定。
type OptionGuardtime struct {
	Interval      *time.Duration // 默认防抖间隔：Interval 内重复操作返回错误
	Tick          *time.Duration // 租约到期检查粒度
	RenewInterval *time.Duration // 续期合并阈值（<=0 表示每次访问都续期）
	Err           error          // 默认重复操作时返回的错误
}

// OptionsGuardtime 创建一个空的 OptionGuardtime，所有字段为零值。
func OptionsGuardtime() *OptionGuardtime {
	return new(OptionGuardtime)
}

// SetInterval 设置默认防抖间隔。
func (this *OptionGuardtime) SetInterval(i time.Duration) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.Interval = &i
	return this
}

// SetTick 设置租约到期检查粒度。
func (this *OptionGuardtime) SetTick(i time.Duration) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.Tick = &i
	return this
}

// SetRenewInterval 设置续期合并阈值。
func (this *OptionGuardtime) SetRenewInterval(i time.Duration) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.RenewInterval = &i
	return this
}

// SetErr 设置默认重复操作时返回的错误。
func (this *OptionGuardtime) SetErr(e error) *OptionGuardtime {
	if this == nil {
		return this
	}
	this.Err = e
	return this
}

// Merge 将多个 OptionGuardtime 依次覆盖合并到当前 OptionGuardtime，仅覆盖非零值。
func (this *OptionGuardtime) Merge(opts ...*OptionGuardtime) *OptionGuardtime {
	if this == nil {
		return this
	}
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

	if opt.Tick != nil {
		this.Tick = opt.Tick
	}

	if opt.RenewInterval != nil {
		this.RenewInterval = opt.RenewInterval
	}

	if opt.Err != nil {
		this.Err = opt.Err
	}
}

// OptionGuardtimeHandle 是 Handle 单次调用的配置项，用于按 key 覆盖实例默认值。
type OptionGuardtimeHandle struct {
	Interval *time.Duration // 本次防抖间隔（覆盖实例默认值）
	Err      error          // 本次重复操作时返回的错误（覆盖实例默认值）
}

// OptionsGuardtimeHandle 创建一个空的 OptionGuardtimeHandle，所有字段为零值。
func OptionsGuardtimeHandle() *OptionGuardtimeHandle {
	return new(OptionGuardtimeHandle)
}

// SetInterval 设置本次调用的防抖间隔。
func (this *OptionGuardtimeHandle) SetInterval(i time.Duration) *OptionGuardtimeHandle {
	if this == nil {
		return this
	}
	this.Interval = &i
	return this
}

// SetErr 设置本次调用重复操作时返回的错误。
func (this *OptionGuardtimeHandle) SetErr(e error) *OptionGuardtimeHandle {
	if this == nil {
		return this
	}
	this.Err = e
	return this
}

// Merge 将多个 OptionGuardtimeHandle 依次覆盖合并到当前对象，仅覆盖非零值。
func (this *OptionGuardtimeHandle) Merge(opts ...*OptionGuardtimeHandle) *OptionGuardtimeHandle {
	if this == nil {
		return this
	}
	for _, opt := range opts {
		this.merge(opt)
	}
	return this
}

func (this *OptionGuardtimeHandle) merge(opt *OptionGuardtimeHandle) {
	if opt == nil {
		return
	}
	if opt.Interval != nil {
		this.Interval = opt.Interval
	}
	if opt.Err != nil {
		this.Err = opt.Err
	}
}
