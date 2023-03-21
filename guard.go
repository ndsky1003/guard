/*
**
该包是一个门卫用法，同时只能有一个使用者消耗一个资源，相当于数据库的行锁
*
*/
package guard

import (
	"fmt"
	"sync"
	"time"
)

/*useage
*	if err := guard.Check(req.ID); err != nil {
		return err
	}
	defer guard.Release(req.ID)
	主要针对资源尚未释放
*/

type guard struct {
	m           sync.Map
	errTemplate string
}

func NewGuard(errTemplate string) *guard {
	return &guard{
		errTemplate: errTemplate,
	}
}

// key 资源标识符
func (this *guard) Check(key any) error {
	if _, ok := this.m.LoadOrStore(key, struct{}{}); ok {
		return fmt.Errorf(this.errTemplate, key)
	}
	return nil
}

func (this *guard) Release(key any) {
	this.m.Delete(key)
}

var g = NewGuard("frequent operation:[%s]")

func Check(key string) error {
	return g.Check(key)
}

func Release(key string) {
	g.Release(key)
}

/*
避免客户端的防抖问题
usage

	gt := NewGuardTime(5*time.Second, errors.New("操作过多"))
	if err := gt.Handle("cc"); err != nil { 检测cc资源是否已经在极端的时间里使用了
		t.Error(err)
	}
*/
type guard_time struct {
	interval    time.Duration
	m           sync.Map
	errTemplate error
}

// 在极端的时间里，操作了这个资源
// interval 时间间隔
// err 时间间隔里的错误
func NewGuardTime(interval time.Duration, err error) *guard_time {
	g := &guard_time{
		interval:    interval,
		errTemplate: err,
	}
	go g.auto_release()
	return g
}

func (this *guard_time) Handle(key any) error {
	now := time.Now()
	if old, ok := this.m.LoadOrStore(key, now); ok {
		if now.Sub(old.(time.Time)) < this.interval {
			return this.errTemplate
		}
	}
	this.m.Store(key, now)
	return nil
}

func (this *guard_time) auto_release() {
	for {
		now := time.Now()
		this.m.Range(func(key, value any) bool {
			if now.Sub(value.(time.Time)) >= this.interval {
				fmt.Println("已经释放了:", key)
				this.m.Delete(key)
			}
			return true
		})
		time.Sleep(1 * time.Second)
	}
}

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
	m              map[string]*bucket
	mutex          sync.Mutex
	bucketLifeTime time.Duration
	checkInterval  time.Duration
}

func (this *guard_wait) gc() {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	now := time.Now()
	for k, v := range this.m {
		if v.lastUse.Add(this.bucketLifeTime).Before(now) && len(v.tickets) == v.cap { //保证所有的入场券已经归还给了桶
			fmt.Println("delete key:", k)
			delete(this.m, k)
		}
	}
}

type bucket struct {
	cap     int
	tickets chan struct{}
	lastUse time.Time
}

func (this *bucket) pushTicket(c int) {
	for i := 0; i < c; i++ {
		this.tickets <- struct{}{}
	}
}
func (this *bucket) ReleaseTicket() {
	this.tickets <- struct{}{}
	this.lastUse = time.Now()
}

// 会阻塞
func (this *bucket) GotTicket() {
	<-this.tickets
}

/*
checkInterval 检查间隔，是否有不用了的桶
bucketLifeTime 桶多久不用就释放掉
*/
func NewGuardWait(checkInterval time.Duration, bucketLifeTime time.Duration) *guard_wait {
	g := &guard_wait{
		m:              make(map[string]*bucket),
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

func (this *guard_wait) GetBucket(key string, cap int) *bucket {
	if key == "" {
		panic("key is empty")
	}
	this.mutex.Lock()
	defer this.mutex.Unlock()
	if v, ok := this.m[key]; ok {
		return v
	}
	t := &bucket{
		cap:     cap,
		tickets: make(chan struct{}, cap),
	}
	t.pushTicket(cap)
	this.m[key] = t
	return t
}

var wg = NewGuardWait(10*time.Second, 30*time.Minute)

func GetBucket(key string) *bucket {
	return wg.GetBucket(key, 1)
}
