package moderation

import (
	"os"
	"strconv"
	"strings"
)

// AdminChecker 判断账号是否具备审核权限。
type AdminChecker interface {
	IsAdmin(accountID uint) bool
}

// StaticAdminChecker 通过白名单 account_id 判定管理员（学习/中小项目常用做法）。
type StaticAdminChecker struct {
	ids map[uint]struct{}
}

// NewStaticAdminChecker 由配置或环境变量注入的管理员 ID 列表构建。
func NewStaticAdminChecker(accountIDs []uint) *StaticAdminChecker {
	m := make(map[uint]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		if id > 0 {
			m[id] = struct{}{}
		}
	}
	return &StaticAdminChecker{ids: m}
}

func (c *StaticAdminChecker) IsAdmin(accountID uint) bool {
	if c == nil || accountID == 0 || len(c.ids) == 0 {
		return false
	}
	_, ok := c.ids[accountID]
	return ok
}

// HasAny 是否配置了至少一名管理员（未配置时 Review 应一律拒绝）。
func (c *StaticAdminChecker) HasAny() bool {
	return c != nil && len(c.ids) > 0
}

// ParseAdminAccountIDs 解析逗号分隔的账号 ID，例如 "1,2,3"。
func ParseAdminAccountIDs(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]uint, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		u, err := strconv.ParseUint(p, 10, 64)
		if err != nil || u == 0 {
			continue
		}
		out = append(out, uint(u))
	}
	return out
}

// AdminCheckerFromEnv 读取环境变量 MODERATION_ADMIN_IDS（优先级由调用方与配置合并决定）。
func AdminCheckerFromEnv() *StaticAdminChecker {
	return NewStaticAdminChecker(ParseAdminAccountIDs(os.Getenv("MODERATION_ADMIN_IDS")))
}
