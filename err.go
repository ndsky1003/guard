package guard

import "errors"

// ErrResourceInUse 表示资源正被占用（行锁冲突）。
var ErrResourceInUse = errors.New("resource is in use")

// ErrTooManyOperations 表示资源在极短时间内被重复操作（防抖触发）。
var ErrTooManyOperations = errors.New("too many operations")
