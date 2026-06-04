package moderation

import "context"

// VideoVisibilityChecker 把 ModerationService 适配为 recommend.VisibilityChecker。
//
// recommend 只关心 video_id；审核模块用 (target_type, target_id) 查 content_reports。
// 在 router 里：recommendService.SetVisibility(NewVideoVisibilityChecker(moderationService))
type VideoVisibilityChecker struct {
	svc *ModerationService
}

func NewVideoVisibilityChecker(svc *ModerationService) *VideoVisibilityChecker {
	return &VideoVisibilityChecker{svc: svc}
}

// IsVisible 实现 recommend.VisibilityChecker：仅检查视频类举报的最新审核结论。
func (v *VideoVisibilityChecker) IsVisible(ctx context.Context, videoID uint) (bool, error) {
	if v == nil || v.svc == nil {
		return true, nil
	}
	return v.svc.IsVisible(ctx, ContentTypeVideo, videoID)
}
