package recommend

import (
	"PulseFeed/internal/feed"
	"context"
	"time"
)

// NewFeedSources 用 Feed 各列表接口适配为多路召回源。
// 在 router 层注入 *feed.FeedService，避免 recommend 包反向依赖 feed 的具体实现细节。
func NewFeedSources(feedSvc *feed.FeedService) []Source {
	if feedSvc == nil {
		return nil
	}
	return []Source{
		NewFuncSource(CandidateSourceLatest, latestFetch(feedSvc)),
		NewFuncSource(CandidateSourcePopularity, popularityFetch(feedSvc)),
		NewFuncSource(CandidateSourceFollowing, followingFetch(feedSvc)),
		// 点赞榜作为补充召回（高互动内容）。
		NewFuncSource(CandidateSourceLikes, likesFetch(feedSvc)),
	}
}

// latestFetch 全站最新：分数用发布时间戳，越新越高。
func latestFetch(feedSvc *feed.FeedService) FetchFunc {
	return func(ctx context.Context, accountID uint, limit int) ([]Candidate, error) {
		resp, err := feedSvc.ListLatest(ctx, limit, time.Time{}, accountID)
		if err != nil {
			return nil, err
		}
		return feedItemsToCandidates(resp.VideoList, CandidateSourceLatest, func(item feed.FeedVideoItem) float64 {
			if item.CreateTime <= 0 {
				return 0
			}
			return float64(item.CreateTime)
		}), nil
	}
}

// popularityFetch 热门榜：用点赞数近似热度（Feed 项无 popularity 字段时）。
func popularityFetch(feedSvc *feed.FeedService) FetchFunc {
	return func(ctx context.Context, accountID uint, limit int) ([]Candidate, error) {
		resp, err := feedSvc.ListByPopularity(ctx, limit, nil, accountID)
		if err != nil {
			return nil, err
		}
		return feedItemsToCandidates(resp.VideoList, CandidateSourcePopularity, func(item feed.FeedVideoItem) float64 {
			return float64(item.LikesCount)
		}), nil
	}
}

// followingFetch 关注流：需登录；未登录时 Feed 返回空列表。
func followingFetch(feedSvc *feed.FeedService) FetchFunc {
	return func(ctx context.Context, accountID uint, limit int) ([]Candidate, error) {
		if accountID == 0 {
			return nil, nil
		}
		resp, err := feedSvc.ListByFollowing(ctx, limit, time.Time{}, accountID)
		if err != nil {
			return nil, err
		}
		return feedItemsToCandidates(resp.VideoList, CandidateSourceFollowing, func(item feed.FeedVideoItem) float64 {
			return float64(item.CreateTime)
		}), nil
	}
}

// likesFetch 点赞榜：补充高 likes 视频。
func likesFetch(feedSvc *feed.FeedService) FetchFunc {
	return func(ctx context.Context, accountID uint, limit int) ([]Candidate, error) {
		resp, err := feedSvc.ListByLikes(ctx, limit, nil, accountID)
		if err != nil {
			return nil, err
		}
		return feedItemsToCandidates(resp.VideoList, CandidateSourceLikes, func(item feed.FeedVideoItem) float64 {
			return float64(item.LikesCount)
		}), nil
	}
}

func feedItemsToCandidates(
	items []feed.FeedVideoItem,
	source CandidateSource,
	scoreFn func(feed.FeedVideoItem) float64,
) []Candidate {
	if len(items) == 0 {
		return nil
	}
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		if item.ID == 0 {
			continue
		}
		out = append(out, Candidate{
			VideoID:   item.ID,
			Source:    source,
			BaseScore: scoreFn(item),
		})
	}
	return out
}
