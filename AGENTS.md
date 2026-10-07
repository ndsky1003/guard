# AGENTS.md

Go 库（`github.com/ndsky1003/guard`），不是应用，无 `main` 入口。单包 `guard`，扁平目录，全部源码在根目录。

## 构建 / 校验

```sh
go build ./...
go test ./...
go vet ./...
```

无 CI、无 lint 配置。

## 模块版本约束

- `go.mod` 中 `go 1.26.0` 是被依赖 `golang.org/x/sync v0.23.0`（其自身声明 `go 1.26.0`）抬上去的，不要手动降低；要降需同时降 `x/sync`。
- 两个依赖：`github.com/ndsky1003/lease`（TTL 资源管理）和 `golang.org/x/sync`（`semaphore.Weighted`）。

## 架构（五种门卫，同包共存）

- `guard.go`：行锁语义，`NewGuard(errs ...error)` + `Check(key)` / `Acquire(key)` / `Release(key)`，基于 `sync.Map.LoadOrStore`，冲突返回实例错误（默认 `ErrResourceInUse`）。
- `guard_time.go`：防抖，`NewGuardTime(...)` + `Handle(key, opts ...*OptionGuardtimeHandle)`，用 `lease.Lease` 记录时间戳，间隔内重复调用返回错误。
- `guard_mutex.go`：`NewGuardMutex(ttl, tick, renewInterval)` + 实例方法 `GetLock(key)`，基于 `lease.Lease`，TTL 强制最小 `1*time.Minute`，`Close()` 停止后台时间轮。
- `guard_rwmutex.go`：`NewGuardRWMutex(ttl, tick, renewInterval)` + 实例方法 `GetRWLock(key)`，同上。
- `guard_wait_sem.go`：限流，`NewGuardWaitSem(checkInterval, ttl)` + 实例方法 `GetSem(key, cap)`，基于 `semaphore.Weighted`，`Sem.Acquire(ctx, n)` / `ReleaseTicket(n)` / `Release()` 分别管理许可与租约引用。
- 仅 `guard.go` 有包级单例 `defaultGuard`（`atomic.Pointer`，`init` 时创建，可用 `SetDefault` 替换）；`guard_mutex` 为纯实例，无全局单例。

## 已知陷阱

- 当前生效的限流 API 在 `guard_wait_sem.go`：`NewGuardWaitSem(checkInterval, ttl)` → `GetSem(key, cap int64) (*Sem, error)` → `Sem.Acquire(ctx, n)` / `Sem.ReleaseTicket(n)` / `Sem.Release()`。
- **两套 guardwait 实现并存**：`guard_wait_cron.go` 是旧版（基于 `sync.Cond`），其类型 `guard_wait_cond`、`NewGuardWaitCond`、`BucketCond` 均为未导出，外部不可用，属历史遗留；改动限流逻辑请改 `guard_wait_sem.go`。

## 风格约定

- 配置风格：`guard_time` 用 Option 模式（`OptionGuardtime` 多字段，setter 返回 `*OptionGuardtime` 支持链式，`Merge` 只覆盖非零值）；`guard.go` 已去掉 Option（单字段，直接传 `error`）；`guard_mutex` 直接传 `ttl/tick/renewInterval` 参数。
- 代码注释与提交信息均为中文。
