package recommend

import "time"

type CandidateSource string

const (
	CandidateSourceLatest      CandidateSource = "latest"
	CandidateSourcePopularity  CandidateSource = "popularity"
	CandidateSourceFollowing   CandidateSource = "following"
	CandidateSourceTagInterest CandidateSource = "tag_interest"
	CandidateSourceLikes       CandidateSource = "likes"
	// CandidateSourceMixed 用于记录"混排后曝光"的来源，区别于单一候选源。
	CandidateSourceMixed CandidateSource = "mixed"
)

type Candidate struct {
	VideoID   uint            `json:"video_id"`
	Source    CandidateSource `json:"source"`
	BaseScore float64         `json:"base_score"`
}

type RankedVideo struct {
	VideoID uint            `json:"video_id"`
	Score   float64         `json:"score"`
	Source  CandidateSource `json:"source"` // 混排时写入，MarkSeen 用
	Reasons []string        `json:"reasons"`
}
type RecommendRequest struct {
	Limit  int    `json:"limit" binding:"omitempty,min=1,max=50"`
	Cursor string `json:"cursor" binding:"omitempty,max=128"`
	Debug  bool   `json:"debug"`
}

type RecommendResponse struct {
	Videos     []RankedVideo `json:"videos"`
	NextCursor string        `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

type RecommendExposure struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	AccountID uint            `gorm:"not null;uniqueIndex:uk_recommend_exposure_account_video,priority:1;index:idx_recommend_exposures_account_time,priority:1" json:"account_id"`
	VideoID   uint            `gorm:"not null;uniqueIndex:uk_recommend_exposure_account_video,priority:2;index" json:"video_id"`
	Source    CandidateSource `gorm:"type:varchar(32);not null" json:"source"`
	Cursor    string          `gorm:"type:varchar(128)" json:"cursor,omitempty"`
	ExposedAt time.Time       `gorm:"not null;index:idx_recommend_exposures_account_time,priority:2" json:"exposed_at"`
	CreatedAt time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

func (s CandidateSource) IsValid() bool {
	switch s {
	case CandidateSourceLatest,
		CandidateSourcePopularity,
		CandidateSourceFollowing,
		CandidateSourceTagInterest,
		CandidateSourceLikes,
		CandidateSourceMixed:
		return true
	default:
		return false
	}
}
