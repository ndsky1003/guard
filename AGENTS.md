# AGENTS.md

Go 库（`github.com/ndsky1003/guard`），不是应用，无 `main` 入口。单包 `guard`，扁平目录，全部源码在根目录。

## 构建 / 校验

```sh
go build ./...
go test ./...   # 目前无任何测试文件
go vet ./...
```

无 CI、无 lint 配置。

## 模块版本约束

- `go.mod` 中 `go 1.26.0` 是被依赖 `golang.org/x/sync v0.23.0`（其自身声明 `go 1.26.0`）抬上去的，不要手动降低；要降需同时降 `x/sync`。
- 两个依赖：`github.com/ndsky1003/lease`（TTL 资源管理）和 `golang.org/x/sync`（`semaphore.Weighted`）。

## 架构（三种门卫，同包共存）

- `guard.go`：行锁语义，`Check(key)` / `Release(key)`，基于 `sync.Map.LoadOrStore`，冲突直接返回 `Option.Err`。
- `guard_time.go`：防抖，`NewGuardTime(...)` + `Handle(key)`，用 `lease.Lease` 记录时间戳，间隔内重复调用返回 `OptionGuardtime.Err`。
- `guard_mutex.go`：`GetLock(key)` / `GetRWLock(key)`，基于 `lease.Lease`，TTL 强制最小 `1*time.Minute`。
- 包级单例 `g`、`gm`、`_gm` 在包初始化时创建（默认 TTL 为 1 小时）。

## 已知陷阱

- **README 已过时**：README 里 `guardwait.GetBucket(...)`、`bucket.GotTicket()`/`ReleaseTicket()` 与当前代码不符。当前生效的限流 API 在 `guard_wait_refactored.go`：`NewGuardWait(checkInterval, bucketLifeTime)` → `GetBucket(key, cap int64) (*Bucket, error)` → `Bucket.Acquire(ctx)` / `Bucket.Release()`。
- **两套 guardwait 实现并存**：`guard_wait_cron.go` 是旧版（基于 `sync.Cond`），其类型 `guard_wait_cond`、`NewGuardWaitCond`、`BucketCond` 均为未导出，外部不可用，属历史遗留；改动限流逻辑请改 `guard_wait_refactored.go`。
- 两处 GC 循环（`guard_wait_refactored.go` 的 `gc()`、`guard_wait_cron.go` 的 `gc()`）用 `fmt.Println` 打印回收日志，会污染日志输出，注意不要新增此类打印。

## 风格约定

- Option 模式统一：`NewXxx(opts ...*OptionXxx)`，`OptionXxx` 的 setter 返回 `*OptionXxx` 支持链式，`Merge` 只覆盖非零值。
- 代码注释与提交信息均为中文。
