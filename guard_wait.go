package guard

import (
	"fmt"
	"sync"
	"time"
)

/*
*门卫发桶（bucket）
*桶中放只有一定数量的令牌(Ticket)
*
*
*问题：不知道chan创建的性能问题
 */
/*usage
*
wg := NewGuardWait(10*time.Second, 35*time.Second) //创建一个发桶的guarder
bucket := wg.GetBucket("ppxia",10)//获取一个桶 ,多大的桶, 这里就相当于同时又10个人可以访问这个资源
bucket.GotTicket() //阻塞获取票圈
defer bucket.ReleaseTicket() //释放票圈
*/
type guard_wait struct {
	m              map[string]*Bucket
	l              sync.Mutex
	bucketLifeTime time.Duration
	checkInterval  time.Duration
}

func (this *guard_wait) gc() {
	this.l.Lock()
	defer this.l.Unlock()
	now := time.Now()
	for k, v := range this.m {
		if v.lastUse.Add(this.bucketLifeTime).Before(now) && len(v.tickets) == v.cap { // 保证所有的入场券已经归还给了桶
			fmt.Println("guard_wait gc delete key:", k)
			delete(this.m, k)
		}
	}
}

type Bucket struct {
	cap     int
	tickets chan struct{}
	lastUse time.Time
}

func (this *Bucket) pushTicket(c int) {
	for i := 0; i < c; i++ {
		this.tickets <- struct{}{}
	}
}

func (this *Bucket) ReleaseTicket() {
	this.tickets <- struct{}{}
	this.lastUse = time.Now()
}

// 会阻塞
func (this *Bucket) GotTicket() *Bucket {
	<-this.tickets
	return this
}

func Wait(key string, cap int) *Bucket {
	bucket := GetBucket(key, cap)
	return bucket.GotTicket()
}

// 只有一个票, 用于单个资源的互斥访问
func WaitAtomic(key string) *Bucket {
	bucket := GetBucket(key, 1)
	return bucket.GotTicket()
}

/*
checkInterval 检查间隔，是否有不用了的桶
bucketLifeTime 桶多久不用就释放掉
*/
func NewGuardWait(checkInterval time.Duration, bucketLifeTime time.Duration) *guard_wait {
	g := &guard_wait{
		m:              make(map[string]*Bucket),
		checkInterval:  checkInterval,
		bucketLifeTime: bucketLifeTime,
	}
	if checkInterval != 0 && bucketLifeTime != 0 {
		go func() {
			for {
				time.Sleep(g.checkInterval)
				g.gc()
			}
		}()
	}
	return g
}

func (this *guard_wait) GetBucket(key string, cap int) *Bucket {
	if key == "" {
		panic("key is empty")
	}
	this.l.Lock()
	defer this.l.Unlock()
	if v, ok := this.m[key]; ok {
		return v
	}
	t := &Bucket{
		cap:     cap,
		tickets: make(chan struct{}, cap),
	}
	t.pushTicket(cap)
	this.m[key] = t
	return t
}

var wg = NewGuardWait(10*time.Second, 30*time.Minute)

func GetBucket(key string, cap int) *Bucket {
	return wg.GetBucket(key, cap)
}
