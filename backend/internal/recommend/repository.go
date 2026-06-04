package recommend

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 是基于 MySQL 的曝光去重实现（recommend_exposures 表），
// 作为 Redis 曝光集合不可用时的降级方案，也可作为持久化曝光审计。
type Repository struct {
	db             *gorm.DB
	exposureWindow time.Duration // 例如 7 * 24 * time.Hour
}

func (r *Repository) exposureSince() time.Time {
	if r.exposureWindow <= 0 {
		return time.Now().Add(-7 * 24 * time.Hour)
	}
	return time.Now().Add(-r.exposureWindow)
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:             db,
		exposureWindow: 7 * 24 * time.Hour,
	}
}

func (r *Repository) AutoMigrate(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("recommend repository is not initialized")
	}
	return r.db.WithContext(ctx).AutoMigrate(&RecommendExposure{})
}

// FilterSeen 过滤掉该用户曾经曝光过的候选视频。
func (r *Repository) FilterSeen(ctx context.Context, accountID uint, candidates []Candidate) ([]Candidate, error) {
	if r == nil || r.db == nil || accountID == 0 || len(candidates) == 0 {
		return candidates, nil
	}

	ids := make([]uint, 0, len(candidates))
	idSet := make(map[uint]struct{}, len(candidates))
	for _, c := range candidates {
		if _, ok := idSet[c.VideoID]; ok {
			continue
		}
		idSet[c.VideoID] = struct{}{}
		ids = append(ids, c.VideoID)
	}

	since := r.exposureSince()
	var seenIDs []uint
	if err := r.db.WithContext(ctx).Model(&RecommendExposure{}).
		Where("account_id = ? AND video_id IN ? AND exposed_at >= ?", accountID, ids, since).
		Distinct().
		Pluck("video_id", &seenIDs).Error; err != nil {
		return nil, err
	}
	if len(seenIDs) == 0 {
		return candidates, nil
	}

	seen := make(map[uint]struct{}, len(seenIDs))
	for _, id := range seenIDs {
		seen[id] = struct{}{}
	}

	kept := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if _, ok := seen[c.VideoID]; ok {
			continue
		}
		kept = append(kept, c)
	}
	return kept, nil
}

// MarkSeen 记录本次返回视频的曝光。
func (r *Repository) MarkSeen(ctx context.Context, accountID uint, videos []RankedVideo, cursor string) error {
	if r == nil || r.db == nil || accountID == 0 || len(videos) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]RecommendExposure, 0, len(videos))
	for _, v := range videos {
		if v.VideoID == 0 {
			continue
		}
		src := v.Source
		if !src.IsValid() {
			src = CandidateSourceMixed
		}

		rows = append(rows, RecommendExposure{
			AccountID: accountID,
			VideoID:   v.VideoID,
			Source:    src,
			Cursor:    cursor,
			ExposedAt: now,
		})
	}
	if len(rows) == 0 {
		return nil
	}

	// MySQL: 冲突时更新 source / exposed_at / cursor，不新增行
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "account_id"},
			{Name: "video_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"source", "exposed_at", "cursor"}),
	}).Create(&rows).Error
}

// DeleteExpired 删除曝光窗口以外的记录（可选定时任务）
func (r *Repository) DeleteExpired(ctx context.Context) (int64, error) {
	cutoff := r.exposureSince()
	result := r.db.WithContext(ctx).
		Where("exposed_at < ?", cutoff).
		Delete(&RecommendExposure{})
	return result.RowsAffected, result.Error
}
