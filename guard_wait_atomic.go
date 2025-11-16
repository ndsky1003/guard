package guard

import (
	"sync"
	"sync/atomic"
	"time"
)

type guard_wait_atomic struct {
	m              map[string]*BucketAtomic
	l              sync.Mutex
	bucketLifeTime time.Duration
	checkInterval  time.Duration
}

type BucketAtomic struct {
	cap       int32
	available int32        // 原子计数器
	lastUse   atomic.Value // time.Time
	waiting   int32        // 等待计数（用于统计）
}

func NewGuardWaitAtomic(checkInterval, bucketLifeTime time.Duration) *guard_wait_atomic {
	g := &guard_wait_atomic{
		m:              make(map[string]*BucketAtomic),
		checkInterval:  checkInterval,
		bucketLifeTime: bucketLifeTime,
	}
	if checkInterval != 0 && bucketLifeTime != 0 {
		go g.gc()
	}
	return g
}

func (g *guard_wait_atomic) GetBucket(key string, cap int) *BucketAtomic {
	g.l.Lock()
	defer g.l.Unlock()

	if v, ok := g.m[key]; ok {
		return v
	}

	bucket := &BucketAtomic{
		cap: int32(cap),
	}
	atomic.StoreInt32(&bucket.available, int32(cap))
	bucket.lastUse.Store(time.Now())
	g.m[key] = bucket
	return bucket
}

func (b *BucketAtomic) GotTicket() *BucketAtomic {
	for {
		current := atomic.LoadInt32(&b.available)
		if current > 0 {
			if atomic.CompareAndSwapInt32(&b.available, current, current-1) {
				b.lastUse.Store(time.Now())
				return b
			}
		}
		atomic.AddInt32(&b.waiting, 1)
		time.Sleep(10 * time.Millisecond) // 短暂休眠避免CPU占用
		atomic.AddInt32(&b.waiting, -1)
	}
}

func (b *BucketAtomic) ReleaseTicket() {
	for {
		current := atomic.LoadInt32(&b.available)
		if current < b.cap {
			if atomic.CompareAndSwapInt32(&b.available, current, current+1) {
				b.lastUse.Store(time.Now())
				return
			}
		} else {
			return // 已经达到容量上限
		}
	}
}

func (b *BucketAtomic) TryGotTicket() bool {
	current := atomic.LoadInt32(&b.available)
	if current > 0 {
		if atomic.CompareAndSwapInt32(&b.available, current, current-1) {
			b.lastUse.Store(time.Now())
			return true
		}
	}
	return false
}
