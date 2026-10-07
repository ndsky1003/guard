# guard

各种事件的门卫，全部线程安全。提供行锁、防抖、命名互斥锁、命名读写锁、限流等并发控制原语。

```sh
go get github.com/ndsky1003/guard
```

## 门卫总览

| 门卫 | 文件 | 用途 | 冲突时的行为 |
|------|------|------|--------------|
| 行锁 Guard | `guard.go` | 同一资源只能被一个使用者占用 | 直接返回错误 |
| 防抖 GuardTime | `guard_time.go` | 避免短时间重复操作（如重复发验证码） | 间隔内重复调用返回错误 |
| 命名互斥锁 GuardMutex | `guard_mutex.go` | 基于 key 的互斥锁，空闲自动回收 | 阻塞等待 |
| 命名读写锁 GuardRWMutex | `guard_rwmutex.go` | 基于 key 的读写锁，空闲自动回收 | 阻塞等待 |
| 限流 GuardWaitSem | `guard_wait_sem.go` | 限制同一 key 同时进入的数量，支持 context 与加权 | 阻塞等待（可超时） |
| 限流 GuardWaitCond | `guard_wait_cron.go` | 基于 sync.Cond 的限流（无 context、固定 1 票） | 阻塞等待 |

---

## 1. 行锁 Guard

相当于数据库的行锁：同一时刻只能有一个使用者占用一个资源，冲突直接返回错误（不等待）。

### 快速使用（包级）

```go
import "github.com/ndsky1003/guard"

if err := guard.Check("resource_id"); err != nil {
    return err // 资源被占用
}
defer guard.Release("resource_id")
```

### 推荐用法（Acquire 返回幂等的 release）

`Check`/`Release` 不做所有权校验，存在 ABA 风险（误删后来占用者的锁）。更安全的是用 `Acquire`，它返回与本次占用绑定的、幂等的释放函数：

```go
release, err := guard.Acquire("resource_id")
if err != nil {
    return err // 资源被占用
}
defer release() // 幂等，重复调用安全
```

### 独立实例与自定义错误

```go
g := guard.NewGuard(errors.New("资源使用中"))

if err := g.Check("resource_id"); err != nil {
    // err 同时满足 errors.Is(err, guard.ErrResourceInUse)
    return err
}
defer g.Release("resource_id")
```

`NewGuard(errs ...error)` 会把传入的错误与 `ErrResourceInUse` 通过 `errors.Join` 合并，因此 `errors.Is(err, guard.ErrResourceInUse)` 始终成立，同时保留自定义错误的可判断性。

### API 说明

| 函数 / 方法 | 说明 |
|-------------|------|
| `Check(key string) error` | 尝试占用；被占用返回错误 |
| `Acquire(key string) (release func(), err error)` | 占用成功返回幂等释放函数（推荐） |
| `Release(key string)` | 释放（无所有权校验，须与成功的 `Check` 一一配对） |
| `NewGuard(errs ...error) *Guard` | 创建独立实例 |
| `SetDefault(g *Guard)` | 替换包级 `Check`/`Acquire`/`Release` 使用的默认实例 |

---

## 2. 防抖 GuardTime

避免客户端在极短时间内重复操作（如重复提交、重复发送验证码）。同一 key 在间隔内被重复 `Handle` 时返回错误，超过间隔后恢复。

### 基本使用

```go
gt := guard.NewGuardTime(guard.OptionsGuardtime().
    SetInterval(5 * time.Second).          // 间隔内重复调用报错
    SetErr(errors.New("操作频繁")))

defer gt.Close()

if err := gt.Handle("user_id"); err != nil {
    // 5 秒内重复请求会走到这里
    return err
}
```

### 不同 key 用不同间隔

```go
gt.Handle("login",    guard.OptionsGuardtimeHandle().SetInterval(5 * time.Second))
gt.Handle("sendcode", guard.OptionsGuardtimeHandle().SetInterval(60 * time.Second))
```

### 实例选项（`OptionGuardtime`）

| Setter | 说明 |
|--------|------|
| `SetInterval(i)` | 防抖间隔，间隔内再次 `Handle` 返回错误（默认 5s） |
| `SetTick(i)` | 租约到期检查粒度（内部时间轮 tick，默认 10s） |
| `SetRenewInterval(i)` | 续期合并阈值（`<=0` 表示每次访问都续期，默认 1s） |
| `SetErr(e)` | 默认重复操作时返回的错误（默认 `ErrTooManyOperations`） |
| `Merge(opts...)` | 合并多个配置，仅覆盖非零值 |

### 单次调用选项（`OptionGuardtimeHandle`）

按 key 覆盖实例默认值：`SetInterval(i)`、`SetErr(e)`。

---

## 3. 命名互斥锁 GuardMutex

基于 key 的互斥锁，同一 key 返回同一把底层锁；空闲超时后自动回收（避免 key 无限增长）。

```go
g := guard.NewGuardMutex(time.Hour, 30*time.Second, time.Second)
defer g.Close()

m := g.GetLock("resource_id")
m.Lock()
defer m.Unlock()
// 临界区
```

参数说明：

| 参数 | 说明 |
|------|------|
| `ttl` | 单把锁的空闲存活时长（低于 1 分钟会强制为 1 分钟） |
| `tick` | 租约到期检查粒度 |
| `renewInterval` | 续期合并阈值（`<=0` 表示每次访问都续期） |

`Lock`/`Unlock` 必须成对调用，`Unlock` 会释放本次 `GetLock` 持有的租约引用。

---

## 4. 命名读写锁 GuardRWMutex

基于 key 的读写锁，与互斥锁同机制，支持读写分离。

```go
g := guard.NewGuardRWMutex(time.Hour, 30*time.Second, time.Second)
defer g.Close()

m := g.GetRWLock("resource_id")

m.Lock()      // 写锁
m.Unlock()

m.RLock()     // 读锁
m.RUnlock()
```

参数同 `GuardMutex`；`Lock`/`Unlock`、`RLock`/`RUnlock` 均须成对调用，`Unlock`/`RUnlock` 会释放本次租约引用。

---

## 5. 限流 GuardWaitSem

高峰时阻塞等待，限制同一 key 同时进入的数量；容量为 1 时即单线程串行。基于 `golang.org/x/sync/semaphore`，支持加权许可与 context 超时。

### 基本使用

```go
gw := guard.NewGuardWaitSem(10*time.Second, 30*time.Minute)
defer gw.Close()

sem, err := gw.GetSem("bucket", 2) // 允许 2 个操作同时进入
if err != nil {
    return err
}
defer sem.Release() // 释放租约引用

if err := sem.Acquire(context.Background(), 1); err != nil {
    return err // 阻塞直到拿到许可，或 ctx 取消
}
defer sem.ReleaseTicket(1) // 释放许可
// 访问资源
```

### 带超时

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

if err := sem.Acquire(ctx, 1); err != nil {
    return err // 1 秒内没拿到许可
}
defer sem.ReleaseTicket(1)
```

### 加权许可（一次获取多个）

容量为 10 的桶，允许一次占用 3 个许可（例如按请求重量级限流）：

```go
sem, _ := gw.GetSem("weighted", 10)
defer sem.Release()

if err := sem.Acquire(ctx, 3); err != nil {
    return err
}
defer sem.ReleaseTicket(3)
```

### API 说明

| 函数 / 方法 | 说明 |
|-------------|------|
| `NewGuardWaitSem(checkInterval, ttl)` | `checkInterval` 信号量过期检查粒度；`ttl` 信号量多久不用就释放 |
| `GetSem(key string, cap int64) (*Sem, error)` | `key` 非空、`cap` 为正；相同 key 与容量复用同一信号量 |
| `Sem.Acquire(ctx, n)` | 阻塞获取 `n` 个许可；`n <= 0` 或 `n > cap` 报错，`ctx` 取消/超时返回错误 |
| `Sem.ReleaseTicket(n)` | 释放 `n` 个许可，与成功的 `Acquire` 成对调用 |
| `Sem.Release()` | 释放本次 `GetSem` 持有的租约引用（幂等） |
| `Close()` | 停止内部时间轮，释放所有信号量 |

使用约定：`GetSem` 后立即 `defer sem.Release()`，`Acquire` 成功后 `defer sem.ReleaseTicket(n)`。

---

## 6. 限流 GuardWaitCond（sync.Cond）

`guard_wait_cron.go` 是基于 `sync.Cond` 的另一套限流实现，与 `GuardWaitSem` 同包并存，两者性能各有所长，按需选择。

### 与 GuardWaitSem 对比

| 维度 | GuardWaitSem | GuardWaitCond |
|------|--------------|---------------|
| 底层实现 | `semaphore.Weighted` | `sync.Cond` |
| context 超时/取消 | ✅ 支持 | ❌ 不支持 |
| 加权许可（一次多个） | ✅ `Acquire(ctx, n)` | ❌ 每次固定 1 票 |
| 非阻塞尝试 | ❌ | ✅ `TryGotTicket()` |
| 无竞争（容量充足） | ~179 ns（更快） | ~225 ns |
| 高竞争（容量 1） | ~213 ns，2 allocs | ~205 ns，0 allocs（更快） |
| 入口返回类型 | 导出 `*GuardWaitSem` | 导出 `*GuardWaitCond` |

**选择建议**：需要超时/加权用 `GuardWaitSem`；纯固定 1 票的高竞争限流可用 `GuardWaitCond`（竞争场景零分配）。

### 使用

```go
g := guard.NewGuardWaitCond(time.Second, time.Minute)
defer g.Close()

b, err := g.GetBucket("k", 2) // 允许 2 个操作同时进入
if err != nil {
    return err
}
defer b.Release()

b.GotTicket()      // 阻塞获取一张票
b.ReleaseTicket()  // 释放一张票

if b.TryGotTicket() { // 非阻塞尝试获取
    b.ReleaseTicket()
}
```

---

## 错误

| 变量 | 含义 |
|------|------|
| `ErrResourceInUse` | 资源正被占用（行锁冲突） |
| `ErrTooManyOperations` | 资源在极短时间内被重复操作（防抖触发） |

---

## 性能基准

以下数据在 Apple M1（8 核）下，用 `go test -run '^$' -bench . -benchmem -benchtime=2s` 测得，按单次操作耗时从快到慢排列。

| Benchmark | 场景 | ns/op | B/op | allocs/op |
|-----------|------|-------|------|-----------|
| `GuardMutexLock` | 命名互斥锁（1000 key） | 131.9 | 60 | 2 |
| `GuardRWMutexRLock` | 读写锁读（1000 key） | 131.9 | 60 | 2 |
| `GuardTimeHandle` | 防抖 | 132.3 | 60 | 2 |
| `GuardRWMutexLock` | 读写锁写（1000 key） | 132.7 | 60 | 2 |
| `GuardWaitSemWeighted` | 限流加权（无竞争） | 178.6 | 0 | 0 |
| `GuardWaitAcquire` | 限流（无竞争） | 178.7 | 0 | 0 |
| `GuardWaitCondTicketContended` | 限流 cond（竞争） | 204.5 | 0 | 0 |
| `GuardWaitAcquireContended` | 限流（竞争） | 212.6 | 175 | 2 |
| `GuardMutexLockContended` | 互斥锁（竞争） | 214.0 | 52 | 2 |
| `GuardRWMutexRLockContended` | 读锁（竞争） | 217.7 | 52 | 2 |
| `GuardWaitCondTicket` | 限流 cond（无竞争） | 224.5 | 0 | 0 |
| `GuardCheck` | 行锁 Check | 248.4 | 76 | 2 |
| `GuardAcquire` | 行锁 Acquire（幂等释放） | 272.2 | 138 | 4 |

要点：

- 所有门卫操作都在 **130–280 ns** 量级。
- **最便宜**：命名互斥锁 / 读写锁 / 防抖（~132 ns），内部是 `lease` + 简单加锁。
- **最贵**：行锁 Guard（248–272 ns），因为走 `sync.Map`，且 `Acquire` 为幂等释放额外创建 `sync.Once` 闭包（4 allocs）。
- **限流无竞争零分配**：sem 加权 / 普通路径 0 alloc（快速路径直接 `cur += n`）。
- **限流竞争对比**：`sync.Cond` 版（205 ns，0 alloc）在竞争下比 sem 版（213 ns，2 alloc）略优且零分配——sem 版每次排队要分配 `waiter` 节点，cond 版走 runtime 信号量无分配。
- **行锁 `Check` 比 `Acquire` 快约 10%**，但 `Acquire` 更安全（幂等释放规避 ABA），是用少量性能换正确性。
