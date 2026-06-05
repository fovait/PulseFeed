package feed

const (
	DefaultFeedLimit = 20
	MaxFeedLimit     = 50
)

func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultFeedLimit
	}
	if limit > MaxFeedLimit {
		return MaxFeedLimit
	}
	return limit
}

type FeedAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type FeedVideoItem struct {
	ID            uint       `json:"id"`
	Author        FeedAuthor `json:"author"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	PlayURL       string     `json:"play_url"`
	CoverURL      string     `json:"cover_url"`
	CreateTime    int64      `json:"create_time"` // Unix seconds
	LikesCount    int64      `json:"likes_count"`
	CommentsCount int64      `json:"comments_count"`
	IsLiked       bool       `json:"is_liked"`
}

// ========================
// 最新流 / 关注流：按时间分页
// ========================

type TimeCursorRequest struct {
	Limit      int   `json:"limit" binding:"omitempty,min=1,max=50"`
	BeforeTime int64 `json:"before_time" binding:"omitempty,min=0"`
}

type TimeCursorResponse struct {
	VideoList      []FeedVideoItem `json:"video_list"`
	NextBeforeTime int64           `json:"next_before_time"`
	HasMore        bool            `json:"has_more"`
}

type ListLatestRequest = TimeCursorRequest
type ListLatestResponse = TimeCursorResponse

type ListByFollowingRequest = TimeCursorRequest
type ListByFollowingResponse = TimeCursorResponse

// ========================
// 点赞榜：按 likes_count + id 游标分页
// ========================

type LikesCursor struct {
	LikesCount int64 `json:"likes_count"`
	ID         uint  `json:"id"`
}

type ListByLikesRequest struct {
	Limit  int          `json:"limit" binding:"omitempty,min=1,max=50"`
	Cursor *LikesCursor `json:"cursor,omitempty"`
}

type ListByLikesResponse struct {
	VideoList  []FeedVideoItem `json:"video_list"`
	NextCursor *LikesCursor    `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}

// ========================
// 热门流：Redis 热榜 as_of + offset 分页
// ========================

type PopularityCursor struct {
	AsOf   int64 `json:"as_of"`
	Offset int   `json:"offset"`
}

type ListByPopularityRequest struct {
	Limit  int               `json:"limit" binding:"omitempty,min=1,max=50"`
	Cursor *PopularityCursor `json:"cursor,omitempty"`
}

type ListByPopularityResponse struct {
	VideoList  []FeedVideoItem   `json:"video_list"`
	NextCursor *PopularityCursor `json:"next_cursor,omitempty"`
	HasMore    bool              `json:"has_more"`
}

type ListByTagRequest struct {
	TagName    string `json:"tag_name" binding:"required"`
	Limit      int    `json:"limit" binding:"omitempty,min=1,max=50"`
	BeforeTime int64  `json:"before_time" binding:"omitempty,min=0"`
}

type ListByTagResponse struct {
	VideoList      []FeedVideoItem `json:"video_list"`
	NextBeforeTime int64           `json:"next_before_time"`
	HasMore        bool            `json:"has_more"`
}
