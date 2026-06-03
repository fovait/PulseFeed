package event

import "errors"

// ErrNotImplemented 保留给尚未接线的占位逻辑（如 MQ 消费循环）。
var ErrNotImplemented = errors.New("event: not implemented")

// ErrInvalidArgument 表示入参缺失或非法（accountID/video_id/idempotency_key 为空等）。
var ErrInvalidArgument = errors.New("event: invalid argument")

// ErrInvalidEventType 表示事件类型不在受支持的枚举内。
var ErrInvalidEventType = errors.New("event: invalid event type")

// ErrDuplicate 表示按 idempotency_key 命中了已存在事件（幂等重放）。
var ErrDuplicate = errors.New("event: duplicate event")

var ErrVideoNotFound = errors.New("event: video not found")
