# guard
各种事件的门卫
都是线程安全的

#### usage

1. guard
> 行锁，直接报错
```go
    resource_identifier = "identifier"
    if err := guard.Check(resource_identifier); err != nil { //检查资源是否在使用
    	return
    }
    guard.Release(resource_identifier) //释放使用中的资源
```
2. guardtime
> 用于避免客户端防抖，使用场景，发短信等
```
	gt := guardtime.NewGuardTime(5*time.Second, errors.New("操作频繁"))
	if err := gt.Handle("cc"); err != nil {
		fmt.Println(err) //5秒内，请求都抛出错误
	}
```
  
3. guardwait
> 限流,高峰的时候，阻塞等待
> 限流为1的时候，就是单线程操作了
```go
	bucket := guardwait.GetBucket("bucket") //获取一个桶,使用的是默认的单线程门卫
	var value int
	for i := 0; i < 100000; i++ {
		go func() {
			bucket.GotTicket()
			defer bucket.ReleaseTicket()
			value++
		}()
	}
	time.Sleep(10e9)
	fmt.Println(value) //最终结果是10000

    //第二种，自定义门卫，以及自定义桶容量
    guarder := guardwait.NewGuardWait(10*time.Second, 30*time.Minute)
	bucket := guarder.GetBucket("bucket", 2) //指定可以有几个操作可以同时进去访问资源
	var value int
	for i := 0; i < 100000; i++ {
		go func() {
			bucket.GotTicket()
			defer bucket.ReleaseTicket()
			value++
		}()
	}
	time.Sleep(10e9)
	fmt.Println(value) //这个结果打到100000的机会几乎为0,因为非线程安全了
```
