package guard

import (
	"fmt"
	"sync"
	"time"
)

type guard_wait_cond struct {
	m              map[string]*BucketCond
	l              sync.Mutex
	bucketLifeTime time.Duration
	checkInterval  time.Duration
}

type BucketCond struct {
	cap       int
	available int        // 可用票数
	cond      *sync.Cond // 条件变量
	mu        sync.Mutex // 保护available
	lastUse   time.Time
	waiting   int // 等待中的goroutine数量
}

func NewGuardWaitCond(checkInterval, bucketLifeTime time.Duration) *guard_wait_cond {
	g := &guard_wait_cond{
		m:              make(map[string]*BucketCond),
		checkInterval:  checkInterval,
		bucketLifeTime: bucketLifeTime,
	}
	if checkInterval != 0 && bucketLifeTime != 0 {
		go g.gc()
	}
	return g
}

func (g *guard_wait_cond) GetBucket(key string, cap int) *BucketCond {
	g.l.Lock()
	defer g.l.Unlock()

	if v, ok := g.m[key]; ok {
		return v
	}

	bucket := &BucketCond{
		cap:       cap,
		available: cap,
		lastUse:   time.Now(),
	}
	bucket.cond = sync.NewCond(&bucket.mu)
	g.m[key] = bucket
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
	b.lastUse = time.Now()
	return b
}

func (b *BucketCond) ReleaseTicket() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.available++
	if b.available > b.cap {
		b.available = b.cap // 防止溢出
	}
	b.lastUse = time.Now()

	// 通知等待的goroutine
	if b.waiting > 0 {
		b.cond.Signal() // 通知一个等待者
	}
}

func (b *BucketCond) TryGotTicket() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.available > 0 {
		b.available--
		b.lastUse = time.Now()
		return true
	}
	return false
}

func (g *guard_wait_cond) gc() {
	for {
		time.Sleep(g.checkInterval)
		g.l.Lock()
		now := time.Now()
		for k, v := range g.m {
			v.mu.Lock()
			if v.lastUse.Add(g.bucketLifeTime).Before(now) && v.available == v.cap && v.waiting == 0 {
				fmt.Println("guard_wait_cond gc delete key:", k)
				delete(g.m, k)
			}
			v.mu.Unlock()
		}
		g.l.Unlock()
	}
}
