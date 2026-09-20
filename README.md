# guard

各种事件的门卫。全部线程安全。

四种门卫：

| 门卫 | 文件 | 用途 |
|------|------|------|
| 行锁 guard | `guard.go` | 资源只能被一个使用者占用，冲突直接报错 |
| 防抖 guardtime | `guard_time.go` | 避免客户端重复操作（如发短信），间隔内重复调用报错 |
| 锁 guardmutex | `guard_mutex.go` | 基于 key 的互斥锁 / 读写锁 |
| 限流 guardwait | `guard_wait_refactored.go` | 高峰时阻塞等待，限制同时进入的数量 |

```sh
go get github.com/ndsky1003/guard
```

---

## 1. 行锁 guard

相当于数据库的行锁：同一时刻只能有一个使用者占用一个资源，冲突直接返回错误。

```go
import "github.com/ndsky1003/guard"

if err := guard.Check("resource_id"); err != nil { // 检查资源是否在使用
    return err // 资源被占用
}
defer guard.Release("resource_id") // 释放资源
```

自定义错误：

```go
g := guard.NewGuard(guard.Options().SetErr(errors.New("资源使用中")))
if err := g.Check("resource_id"); err != nil {
    return err
}
defer g.Release("resource_id")
```

- 包级 `Check(key string, opts ...*Option) error` / `Release(key string)`。
- `NewGuard(opts ...*Option)` 创建独立实例，`(*guard).Check` / `Release` 接受任意类型 key。
- **注意**：行锁无自动过期，`Check` 后必须配对 `Release`，否则资源永久占用。

---

## 2. 防抖 guardtime

避免客户端在极短时间内重复操作（如发送验证码）。

```go
gt := guard.NewGuardTime(guard.OptionsGuardtime().
    SetInterval(5 * time.Second).      // 间隔内重复调用报错
    SetErr(errors.New("操作频繁")))

if err := gt.Handle("user_id"); err != nil {
    // 5 秒内重复请求会走到这里
    fmt.Println(err)
}
```

选项：

| 字段 | 说明 |
|------|------|
| `SetInterval(i)` | 防抖间隔，间隔内再次 `Handle` 返回 `Err` |
| `SetClearInterval(i)` | 过期记录的清理粒度（内部时间轮 tick） |
| `SetErr(e)` | 自定义错误 |

`Handle(key any, opts ...*OptionGuardtime)` 首次调用成功，间隔内重复调用返回错误，超过间隔后恢复。

---

## 3. 锁 guardmutex

基于 key 的互斥锁和读写锁，key 级别的锁自动过期回收（TTL 最小 1 分钟）。

### 互斥锁

```go
m := guard.GetLock("resource_id")
m.Lock()
defer m.Unlock()
// 临界区
```

### 读写锁

```go
m := guard.GetRWLock("resource_id")
m.Lock()    // 写锁
m.Unlock()

m.RLock()   // 读锁
m.RUnlock()
```

- `GetLock(key)` / `GetRWLock(key)` 使用包级单例，默认 TTL 1 小时。
- `Lock`/`Unlock`（读写锁还有 `RLock`/`RUnlock`）必须成对调用，`Unlock` 会释放本次租约引用。

---

## 4. 限流 guardwait

高峰时阻塞等待，控制同时进入的数量；容量为 1 时即单线程串行。

```go
gw := guard.NewGuardWait(10*time.Second, 30*time.Minute)
defer gw.Close()

bucket, err := gw.GetBucket("bucket", 2) // 允许 2 个操作同时进入
if err != nil {
    return err
}

if err := bucket.Acquire(context.Background()); err != nil { // 阻塞直到拿到许可，或 ctx 取消
    return err
}
defer bucket.Release()
// 访问资源
```

- `NewGuardWait(checkInterval, bucketLifeTime)`：
  - `checkInterval` 桶过期检查粒度（时间轮 tick）。
  - `bucketLifeTime` 桶多久不使用就自动释放。
- `GetBucket(key string, cap int64) (*Bucket, error)`：`key` 非空、`cap` 为正。
- `Acquire(ctx)` 阻塞直到获取许可，`ctx` 取消/超时返回错误；`Release()` 释放许可。
- `Close()` 停止内部时间轮，释放所有桶。

带超时的限流：

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
if err := bucket.Acquire(ctx); err != nil {
    return err // 1 秒内没拿到许可
}
defer bucket.Release()
```
