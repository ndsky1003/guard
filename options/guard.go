package options

type GuardOptions struct {
	Err error
}

func Guard() *GuardOptions {
	return new(GuardOptions)
}

func (this *GuardOptions) SetErr(e error) *GuardOptions {
	if this == nil {
		return this
	}
	this.Err = e
	return this
}

func (this *GuardOptions) Merge(opts ...*GuardOptions) *GuardOptions {
	for _, opt := range opts {
		this.merge(opt)
	}
	return this
}

func (this *GuardOptions) merge(opt *GuardOptions) {
	if opt.Err != nil {
		this.Err = opt.Err
	}
}
