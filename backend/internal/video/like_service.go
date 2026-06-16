package video

import (
	rabbitmq "PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrLockConflict = errors.New("operation in progress, please retry")

const LikePopularityDelta int64 = 1
const LikeLockTTL = 5 * time.Second

type LikeService struct {
	repo         *LikeRepository
	VideoRepo    *VideoRepository
	cache        *rediscache.Client
	likeMQ       *rabbitmq.LikeMQ
	popularityMQ *rabbitmq.PopularityMQ
}

func NewLikeService(
	repo *LikeRepository,
	videoRepo *VideoRepository,
	cache *rediscache.Client,
	likeMQ *rabbitmq.LikeMQ,
	popularityMQ *rabbitmq.PopularityMQ,
) *LikeService {
	return &LikeService{
		repo:         repo,
		VideoRepo:    videoRepo,
		cache:        cache,
		likeMQ:       likeMQ,
		popularityMQ: popularityMQ,
	}
}

func (s *LikeService) likeLockKey(prefix string, videoID, accountID uint) string {
	return s.cache.Key("like:lock:%s:%d:%d", prefix, videoID, accountID)
}

// 最终一致性，不确保处理成功，只是丢给消息队列去异步处理
func (s *LikeService) Like(ctx context.Context, like *Like) error {
	if like == nil {
		return ErrLikeNil
	}

	if like.VideoID == 0 || like.AccountID == 0 {
		return ErrInvalidLike
	}

	if s.VideoRepo != nil {
		ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrVideoNotFound
		}
	}

	if s.cache != nil {
		lockKey := s.likeLockKey("like", like.VideoID, like.AccountID)
		lockCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		token, ok, err := s.cache.Lock(lockCtx, lockKey, LikeLockTTL)
		if err != nil || !ok {
			return ErrLockConflict
		}
		defer func() {
			unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer unlockCancel()
			s.cache.Unlock(unlockCtx, lockKey, token)
		}()
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if isLiked {
		return ErrAlreadyLiked
	}

	like.CreatedAt = time.Now()

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when like MQ publish fails.
	if !mysqlEnqueued {
		err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Select("id").First(&Video{}, like.VideoID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrVideoNotFound
				}
				return err
			}
			if err := tx.Create(like).Error; err != nil {
				if isDupKey(err) {
					return ErrAlreadyLiked
				}
				return err
			}
			if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
		})
		if err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, like.VideoID, LikePopularityDelta)
	}
	return nil
}

func (s *LikeService) UnLike(ctx context.Context, like *Like) error {
	if like == nil {
		return ErrLikeNil
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		return ErrInvalidLike
	}

	if s.VideoRepo != nil {
		ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrVideoNotFound
		}
	}

	if s.cache != nil {
		lockKey := s.likeLockKey("unlike", like.VideoID, like.AccountID)
		lockCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		token, ok, err := s.cache.Lock(lockCtx, lockKey, LikeLockTTL)
		if err != nil || !ok {
			return ErrLockConflict
		}
		defer func() {
			unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer unlockCancel()
			s.cache.Unlock(unlockCtx, lockKey, token)
		}()
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !isLiked {
		return ErrNotLiked
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Unlike(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when like MQ publish fails.
	if !mysqlEnqueued {
		err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			del := tx.Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).Delete(&Like{})
			if del.Error != nil {
				return del.Error
			}
			if del.RowsAffected == 0 {
				return ErrNotLiked
			}

			if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count - 1, 0)")).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("popularity", gorm.Expr("GREATEST(popularity - 1, 0)")).Error
		})
		if err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, like.VideoID, -LikePopularityDelta)
	}
	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	return s.repo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	return s.repo.ListLikedVideos(ctx, accountID)
}
