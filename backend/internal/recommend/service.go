package recommend

import (
	"context"
	"log"
	"sort"
)

const (
	DefaultLimit = 20
	MaxLimit     = 50

	// 每个候选源单次召回上限，防止过滤后仍不足时无限放大。
	maxFetchPerSource = 200
	// 召回轮次：过滤掉已曝光/不可见后不够一页时，加大 fetchLimit 再试。
	maxFetchRounds = 3
)

// Source 是一个候选源（如最新流、热榜、关注流、标签兴趣）。
type Source interface {
	Name() CandidateSource
	Fetch(ctx context.Context, accountID uint, limit int) ([]Candidate, error)
}

// Ranker 将合并后的候选打分排序成最终结果。
type Ranker interface {
	Rank(ctx context.Context, accountID uint, candidates []Candidate) ([]RankedVideo, error)
}

// ExposureStore 负责曝光去重：返回前过滤已曝光，返回后记录新曝光。
type ExposureStore interface {
	FilterSeen(ctx context.Context, accountID uint, candidates []Candidate) ([]Candidate, error)
	// cursor 写入 recommend_exposures，便于排查会话；与请求里的 RecommendRequest.Cursor 一致。
	MarkSeen(ctx context.Context, accountID uint, videos []RankedVideo, cursor string) error
}

// VisibilityChecker 用于在推荐流中过滤被审核隐藏/拒绝的内容（可选）。
type VisibilityChecker interface {
	IsVisible(ctx context.Context, videoID uint) (bool, error)
}

// FetchFunc 把任意取候选逻辑适配成 Source，避免 recommend 反向依赖 feed 等包。
type FetchFunc func(ctx context.Context, accountID uint, limit int) ([]Candidate, error)

type FuncSource struct {
	name  CandidateSource
	fetch FetchFunc
}

func NewFuncSource(name CandidateSource, fetch FetchFunc) *FuncSource {
	return &FuncSource{name: name, fetch: fetch}
}

func (s *FuncSource) Name() CandidateSource { return s.name }

func (s *FuncSource) Fetch(ctx context.Context, accountID uint, limit int) ([]Candidate, error) {
	if s == nil || s.fetch == nil {
		return nil, nil
	}
	return s.fetch(ctx, accountID, limit)
}

// ScoreRanker 是默认排序器：复用 rankByScore。
type ScoreRanker struct{}

func NewScoreRanker() *ScoreRanker { return &ScoreRanker{} }

func (r *ScoreRanker) Rank(ctx context.Context, accountID uint, candidates []Candidate) ([]RankedVideo, error) {
	return rankByScore(candidates), nil
}

type RecommendService struct {
	sources    []Source
	ranker     Ranker
	exposure   ExposureStore
	visibility VisibilityChecker
}

func NewRecommendService(sources []Source, ranker Ranker, exposure ExposureStore) *RecommendService {
	return &RecommendService{sources: sources, ranker: ranker, exposure: exposure}
}

// SetVisibility 注入可选的可见性过滤器（如内容审核模块）。
func (s *RecommendService) SetVisibility(v VisibilityChecker) {
	s.visibility = v
}

// Recommend 执行一次推荐，整体流程：
//
//  1. 多源召回（可多轮加大 fetchLimit）
//  2. 可见性过滤 -> 曝光去重（FilterSeen）
//  3. 打分排序 -> 可选 keyset 游标截断
//  4. limit+1 判断 has_more，截断返回
//  5. MarkSeen 记录本页曝光（与 event/impression 埋点分工不同）
func (s *RecommendService) Recommend(ctx context.Context, accountID uint, req RecommendRequest) (RecommendResponse, error) {
	limit := NormalizeLimit(req.Limit)
	// 与 feed 一致：多排 1 条，用于判断 HasMore，不直接返回给客户端。
	queryLimit := limit + 1

	var ranked []RankedVideo
	fetchLimit := queryLimit * 2

	for round := 0; round < maxFetchRounds; round++ {
		if fetchLimit > maxFetchPerSource {
			fetchLimit = maxFetchPerSource
		}

		candidates := s.fetchAllSources(ctx, accountID, fetchLimit)
		candidates = s.applyPipeline(ctx, accountID, candidates)

		ranked = s.rank(ctx, accountID, candidates)
		// 客户端带上页游标时，在当次排序结果上跳过已返回过的尾部之前的数据。
		ranked = applyRecommendCursor(ranked, req.Cursor)

		if len(ranked) >= queryLimit {
			break
		}
		// 过滤后不够一页：加大每源召回量再试（仍受 maxFetchPerSource 限制）。
		if fetchLimit >= maxFetchPerSource {
			break
		}
		fetchLimit *= 2
	}

	page, hasMore := trimRankedForPage(ranked, limit)

	if s.exposure != nil && len(page) > 0 {
		if err := s.exposure.MarkSeen(ctx, accountID, page, req.Cursor); err != nil {
			log.Printf("recommend: mark seen failed (account=%d): %v", accountID, err)
		}
	}

	resp := RecommendResponse{
		Videos:  page,
		HasMore: hasMore,
	}
	if hasMore && len(page) > 0 {
		last := page[len(page)-1]
		resp.NextCursor = EncodeRecommendCursor(last.Score, last.VideoID)
	}
	return resp, nil
}

// fetchAllSources 从所有候选源拉取候选（单源失败不影响其他源）。
func (s *RecommendService) fetchAllSources(ctx context.Context, accountID uint, fetchLimit int) []Candidate {
	var candidates []Candidate
	for _, src := range s.sources {
		if src == nil {
			continue
		}
		cs, err := src.Fetch(ctx, accountID, fetchLimit)
		if err != nil {
			log.Printf("recommend: source %s fetch failed: %v", src.Name(), err)
			continue
		}
		candidates = append(candidates, cs...)
	}
	return candidates
}

// applyPipeline 可见性 + 曝光去重。
func (s *RecommendService) applyPipeline(ctx context.Context, accountID uint, candidates []Candidate) []Candidate {
	if len(candidates) == 0 {
		return candidates
	}
	if s.visibility != nil {
		candidates = s.filterVisible(ctx, candidates)
	}
	if s.exposure != nil && len(candidates) > 0 {
		filtered, err := s.exposure.FilterSeen(ctx, accountID, candidates)
		if err != nil {
			// 降级：曝光表异常时不阻断推荐，仅跳过过滤（与 feed Redis 降级思路一致）。
			log.Printf("recommend: filter seen failed (account=%d), skip: %v", accountID, err)
			return candidates
		}
		candidates = filtered
	}
	return candidates
}

func (s *RecommendService) rank(ctx context.Context, accountID uint, candidates []Candidate) []RankedVideo {
	if s.ranker != nil {
		r, err := s.ranker.Rank(ctx, accountID, candidates)
		if err != nil {
			log.Printf("recommend: rank failed: %v", err)
			return nil
		}
		return r
	}
	return rankByScore(candidates)
}

func (s *RecommendService) filterVisible(ctx context.Context, candidates []Candidate) []Candidate {
	kept := make([]Candidate, 0, len(candidates))
	cache := make(map[uint]bool, len(candidates))
	for _, c := range candidates {
		visible, ok := cache[c.VideoID]
		if !ok {
			v, err := s.visibility.IsVisible(ctx, c.VideoID)
			if err != nil {
				v = true // 查询失败时保留，避免误伤正常内容
			}
			visible = v
			cache[c.VideoID] = v
		}
		if visible {
			kept = append(kept, c)
		}
	}
	return kept
}

// trimRankedForPage 使用 limit+1 策略判断 HasMore。
func trimRankedForPage(ranked []RankedVideo, limit int) ([]RankedVideo, bool) {
	if len(ranked) > limit {
		return ranked[:limit], true
	}
	return ranked, false
}

// rankByScore 合并多源候选：同 videoID 分数累加，Reasons 记录参与源。
// Source 取「单源贡献 BaseScore 最大」的来源，供 MarkSeen 写入 recommend_exposures。
func rankByScore(candidates []Candidate) []RankedVideo {
	type agg struct {
		score       float64
		primarySrc  CandidateSource
		primaryPart float64
		reasons     []string
		seen        map[CandidateSource]bool
	}
	byID := make(map[uint]*agg)
	order := make([]uint, 0, len(candidates))

	for _, c := range candidates {
		if c.VideoID == 0 {
			continue
		}
		a := byID[c.VideoID]
		if a == nil {
			a = &agg{seen: map[CandidateSource]bool{}}
			byID[c.VideoID] = a
			order = append(order, c.VideoID)
		}
		a.score += c.BaseScore
		// 主来源：谁给的 BaseScore 最高（用于曝光表 source 字段）。
		if c.BaseScore > a.primaryPart {
			a.primaryPart = c.BaseScore
			a.primarySrc = c.Source
		}
		if !a.seen[c.Source] {
			a.seen[c.Source] = true
			a.reasons = append(a.reasons, string(c.Source))
		}
	}

	out := make([]RankedVideo, 0, len(order))
	for _, id := range order {
		a := byID[id]
		src := a.primarySrc
		if !src.IsValid() {
			src = CandidateSourceMixed
		}
		out = append(out, RankedVideo{
			VideoID: id,
			Score:   a.score,
			Source:  src,
			Reasons: a.reasons,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].VideoID < out[j].VideoID
	})
	return out
}

func NormalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultLimit
	case limit > MaxLimit:
		return MaxLimit
	default:
		return limit
	}
}
