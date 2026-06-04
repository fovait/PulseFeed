package moderation

import "errors"

// ErrNotImplemented 保留给尚未接线的占位逻辑。
var ErrNotImplemented = errors.New("moderation: not implemented")

// ErrInvalidArgument 表示入参缺失或非法。
var ErrInvalidArgument = errors.New("moderation: invalid argument")

// ErrInvalidTargetType 表示举报目标类型不受支持。
var ErrInvalidTargetType = errors.New("moderation: invalid target type")

// ErrInvalidStatus 表示审核结论不是合法的审核状态。
var ErrInvalidStatus = errors.New("moderation: invalid review status")

// ErrForbidden 表示当前账号无审核权限。
var ErrForbidden = errors.New("moderation: forbidden")
