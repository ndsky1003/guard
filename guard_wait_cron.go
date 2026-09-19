package guard

import (
	"sync"
	"time"

	"github.com/ndsky1003/lease"
)

type guard_wait_cond struct {
	l              *lease.Lease[string, *BucketCond]
	mu             sync.Mutex
	bucketLifeTime time.Duration
	checkInterval  time.Duration
}

type BucketCond struct {
	cap       int
	available int        // 可用票数
	cond      *sync.Cond // 条件变量
	mu        sync.Mutex // 保护available
	waiting   int        // 等待中的goroutine数量
	touch     func()     // 记录一次访问，续期租约
}

func NewGuardWaitCond(checkInterval, bucketLifeTime time.Duration) *guard_wait_cond {
	if checkInterval <= 0 {
		checkInterval = time.Second
	}
	g := &guard_wait_cond{
		checkInterval:  checkInterval,
		bucketLifeTime: bucketLifeTime,
	}
	g.l = lease.NewWithOptions(lease.Options[string, *BucketCond]{
		Tick: checkInterval,
	})
	return g
}

func (g *guard_wait_cond) GetBucket(key string, cap int) *BucketCond {
	g.mu.Lock()
	defer g.mu.Unlock()

	if v, release, ok := g.l.Get(key); ok {
		defer release()
		return v
	}

	bucket := &BucketCond{
		cap:       cap,
		available: cap,
	}
	bucket.cond = sync.NewCond(&bucket.mu)
	bucket.touch = func() {
		_, release, ok := g.l.Get(key)
		if ok {
			release()
		}
	}
	g.l.Set(key, bucket, g.bucketLifeTime)
	return bucket
}

func (b *BucketCond) GotTicket() *BucketCond {
	b.mu.Lock()
	defer b.mu.Unlock()

	for b.available <= 0 {
		b.waiting++
		b.cond.Wait() // 等待条件满足
		b.waiting--
	}

	b.available--
	if b.touch != nil {
		b.touch()
	}
	return b
}

func (b *BucketCond) ReleaseTicket() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.available++
	if b.available > b.cap {
		b.available = b.cap // 防止溢出
	}

	// 通知等待的goroutine
	if b.waiting > 0 {
		b.cond.Signal() // 通知一个等待者
	}
	if b.touch != nil {
		b.touch()
	}
}

func (b *BucketCond) TryGotTicket() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.available > 0 {
		b.available--
		if b.touch != nil {
			b.touch()
		}
		return true
	}
	return false
}
