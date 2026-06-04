package recommend

import (
	"fmt"
	"strconv"
	"strings"
)

// 游标格式：v1:<score>:<video_id>
// 与 rankByScore 的排序一致：Score 降序，同分 VideoID 升序。
// 下一页只保留「排在游标之后」的视频（分数更低，或同分且 ID 更大）。
const cursorPrefix = "v1:"

// EncodeRecommendCursor 根据本页最后一条结果生成 next_cursor。
func EncodeRecommendCursor(score float64, videoID uint) string {
	return fmt.Sprintf("%s%.6f:%d", cursorPrefix, score, videoID)
}

// DecodeRecommendCursor 解析客户端回传的 cursor。
func DecodeRecommendCursor(cursor string) (score float64, videoID uint, ok bool) {
	if cursor == "" || !strings.HasPrefix(cursor, cursorPrefix) {
		return 0, 0, false
	}
	rest := strings.TrimPrefix(cursor, cursorPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	s, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return 0, 0, false
	}
	return s, uint(id), true
}

// applyRecommendCursor 在当次排序结果上继续做 keyset 截断。
// 说明：推荐每请求会重新召回，游标不能保证像 Feed 时间线那样绝对稳定；
// 主要与 FilterSeen 配合，避免同一排序批次内重复；跨页仍依赖曝光去重。
func applyRecommendCursor(ranked []RankedVideo, cursor string) []RankedVideo {
	score, vid, ok := DecodeRecommendCursor(cursor)
	if !ok {
		return ranked
	}
	out := make([]RankedVideo, 0, len(ranked))
	for _, v := range ranked {
		if v.Score < score || (v.Score == score && v.VideoID > vid) {
			out = append(out, v)
		}
	}
	return out
}
