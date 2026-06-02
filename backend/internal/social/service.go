package social

import (
	"PulseFeed/internal/account"
	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"context"
	"errors"
	"log"
	"time"
)

type SocialService struct {
	repo        *SocialRepository
	accountRepo *account.AccountRepository
	socialMQ    *rabbitmq.SocialMQ
	cache       *rediscache.Client
}

func NewSocialService(
	repo *SocialRepository,
	accountRepo *account.AccountRepository,
	socialMQ *rabbitmq.SocialMQ,
	cache *rediscache.Client,
) *SocialService {
	return &SocialService{
		repo:        repo,
		accountRepo: accountRepo,
		socialMQ:    socialMQ,
		cache:       cache,
	}
}

// validateSocial 用于校验关注关系是否合法。
//
// 检查内容：
// 1. social 不能为 nil
// 2. follower_id 和 vlogger_id 必须有效
// 3. 不能关注自己
// 4. follower 和 vlogger 都必须存在
func (s *SocialService) validateSocial(ctx context.Context, social *Social) error {
	if s == nil {
		return errors.New("social service is nil")
	}
	if s.repo == nil {
		return errors.New("social repository is nil")
	}
	if s.accountRepo == nil {
		return errors.New("account repository is nil")
	}
	if social == nil {
		return errors.New("social is nil")
	}
	if social.FollowerID == 0 || social.VloggerID == 0 {
		return errors.New("follower_id and vlogger_id are required")
	}
	if social.FollowerID == social.VloggerID {
		return errors.New("can not follow self")
	}

	// 检查关注者是否存在。
	if _, err := s.accountRepo.FindByID(ctx, social.FollowerID); err != nil {
		return err
	}

	// 检查被关注者是否存在。
	if _, err := s.accountRepo.FindByID(ctx, social.VloggerID); err != nil {
		return err
	}

	return nil
}

// invalidateFollowingFeedCache 删除某个用户的关注流缓存。
//
// 当用户关注或取消关注别人后，他的关注流会发生变化。
// 所以需要删除这个用户的 feed:following 缓存。
func (s *SocialService) invalidateFollowingFeedCache(accountID uint) {
	if s.cache == nil || accountID == 0 {
		return
	}

	// 注意：这里要和 FeedService 里的关注流缓存 key 保持一致。
	//
	// FeedService 里的 key：
	// feed:following:account=%d:limit=%d:before=%d
	//
	// 所以这里删除：
	// feed:following:account={accountID}:*
	pattern := s.cache.Key("feed:following:account=%d:*", accountID)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := s.cache.DelByPattern(ctx, pattern); err != nil {
		log.Printf("failed to invalidate following feed cache: accountID=%d, err=%v", accountID, err)
	}
}

// Follow 关注用户。
//
// 设计成幂等：
// 1. 如果之前没关注，则创建关注关系
// 2. 如果已经关注过，则直接返回 nil
//
// 只有真正新增关注关系时，才需要：
// 1. 删除关注流缓存
// 2. 发送 MQ 事件
func (s *SocialService) Follow(ctx context.Context, social *Social) error {
	if err := s.validateSocial(ctx, social); err != nil {
		return err
	}

	created, err := s.repo.FollowIgnoreDuplicate(ctx, social)
	if err != nil {
		return err
	}

	// created == false 表示之前已经关注过。
	// 幂等设计下，重复关注直接认为成功。
	if !created {
		return nil
	}

	// 关注关系变化后，当前用户的关注流缓存需要失效。
	s.invalidateFollowingFeedCache(social.FollowerID)

	// MQ 只是附加事件，用于通知、推荐、行为日志等。
	// DB 已经写成功，所以 MQ 失败不影响关注结果。
	if s.socialMQ != nil {
		if err := s.socialMQ.Follow(ctx, social.FollowerID, social.VloggerID); err != nil {
			log.Printf("social MQ Follow publish failed: followerID=%d, vloggerID=%d, err=%v",
				social.FollowerID,
				social.VloggerID,
				err,
			)
		}
	}

	return nil
}

// Unfollow 取消关注。
//
// 设计成幂等：
// 1. 如果之前关注过，则删除关注关系
// 2. 如果原本就没关注，也直接返回 nil
//
// 只有真正删除关注关系时，才需要：
// 1. 删除关注流缓存
// 2. 发送 MQ 事件
func (s *SocialService) Unfollow(ctx context.Context, social *Social) error {
	if err := s.validateSocial(ctx, social); err != nil {
		return err
	}

	deleted, err := s.repo.DeleteByFollowerAndVlogger(
		ctx,
		social.FollowerID,
		social.VloggerID,
	)
	if err != nil {
		return err
	}

	// deleted == false 表示原本就没有关注关系。
	// 幂等设计下，重复取消关注也认为成功。
	if !deleted {
		return nil
	}

	// 取消关注后，当前用户的关注流缓存也要失效。
	s.invalidateFollowingFeedCache(social.FollowerID)

	// MQ 失败只记录日志，不影响主业务。
	if s.socialMQ != nil {
		if err := s.socialMQ.UnFollow(ctx, social.FollowerID, social.VloggerID); err != nil {
			log.Printf("social MQ UnFollow publish failed: followerID=%d, vloggerID=%d, err=%v",
				social.FollowerID,
				social.VloggerID,
				err,
			)
		}
	}

	return nil
}

// ListFollowers 查询某个用户的粉丝列表。
// vloggerID 表示被关注的人，也就是要查询谁的粉丝。
func (s *SocialService) ListFollowers(
	ctx context.Context,
	vloggerID uint,
) ([]*account.Account, error) {
	if s == nil || s.repo == nil || s.accountRepo == nil {
		return nil, errors.New("social service is not initialized")
	}
	if vloggerID == 0 {
		return nil, errors.New("vlogger_id is required")
	}

	// 先确认这个用户存在。
	if _, err := s.accountRepo.FindByID(ctx, vloggerID); err != nil {
		return nil, err
	}

	return s.repo.ListFollowers(ctx, vloggerID)
}

// ListFollowing 查询某个用户关注了哪些人。
// followerID 表示关注者，也就是查询这个用户关注了谁。
func (s *SocialService) ListFollowing(
	ctx context.Context,
	followerID uint,
) ([]*account.Account, error) {
	if s == nil || s.repo == nil || s.accountRepo == nil {
		return nil, errors.New("social service is not initialized")
	}
	if followerID == 0 {
		return nil, errors.New("follower_id is required")
	}

	// 先确认这个用户存在。
	if _, err := s.accountRepo.FindByID(ctx, followerID); err != nil {
		return nil, err
	}

	return s.repo.ListFollowing(ctx, followerID)
}

// CountFollowers 统计某个用户的粉丝数。
func (s *SocialService) CountFollowers(
	ctx context.Context,
	vloggerID uint,
) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("social service is not initialized")
	}
	if vloggerID == 0 {
		return 0, errors.New("vlogger_id is required")
	}

	return s.repo.CountFollowers(ctx, vloggerID)
}

// CountFollowing 统计某个用户关注了多少人。
func (s *SocialService) CountFollowing(
	ctx context.Context,
	followerID uint,
) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("social service is not initialized")
	}
	if followerID == 0 {
		return 0, errors.New("follower_id is required")
	}

	return s.repo.CountFollowing(ctx, followerID)
}

// IsFollowed 判断 followerID 是否关注了 vloggerID。
func (s *SocialService) IsFollowed(
	ctx context.Context,
	social *Social,
) (bool, error) {
	if err := s.validateSocial(ctx, social); err != nil {
		return false, err
	}

	return s.repo.IsFollowed(ctx, social)
}

// GetSocialCounts 获取某个用户的粉丝数和关注数。
func (s *SocialService) GetSocialCounts(
	ctx context.Context,
	accountID uint,
) (SocialCounts, error) {
	if s == nil || s.repo == nil {
		return SocialCounts{}, errors.New("social service is not initialized")
	}
	if accountID == 0 {
		return SocialCounts{}, errors.New("account_id is required")
	}

	followerCount, err := s.repo.CountFollowers(ctx, accountID)
	if err != nil {
		return SocialCounts{}, err
	}

	followingCount, err := s.repo.CountFollowing(ctx, accountID)
	if err != nil {
		return SocialCounts{}, err
	}

	return SocialCounts{
		FollowerCount: followerCount,
		VloggerCount:  followingCount,
	}, nil
}
