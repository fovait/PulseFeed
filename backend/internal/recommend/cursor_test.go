package recommend

import "testing"

func TestRecommendCursorRoundTrip(t *testing.T) {
	cur := EncodeRecommendCursor(12.5, 42)
	score, id, ok := DecodeRecommendCursor(cur)
	if !ok || score != 12.5 || id != 42 {
		t.Fatalf("decode failed: ok=%v score=%v id=%v", ok, score, id)
	}
}

func TestApplyRecommendCursor(t *testing.T) {
	ranked := []RankedVideo{
		{VideoID: 1, Score: 10},
		{VideoID: 2, Score: 10},
		{VideoID: 3, Score: 5},
	}
	cur := EncodeRecommendCursor(10, 2)
	got := applyRecommendCursor(ranked, cur)
	if len(got) != 1 || got[0].VideoID != 3 {
		t.Fatalf("unexpected after cursor: %+v", got)
	}
}

func TestTrimRankedForPage(t *testing.T) {
	page, more := trimRankedForPage([]RankedVideo{{VideoID: 1}, {VideoID: 2}, {VideoID: 3}}, 2)
	if !more || len(page) != 2 {
		t.Fatalf("want 2 items and hasMore, got len=%d more=%v", len(page), more)
	}
}
