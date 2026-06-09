package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMetricsMiddlewareRecordsAndExposes 验证:中间件采集到请求后,/metrics 能暴露
// 计数器/直方图/在途数三类指标,且标签维度(method/path/status)正确。
func TestMetricsMiddlewareRecordsAndExposes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MetricsMiddleware())
	r.GET("/metrics", MetricsHandler())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	// 打一次业务请求,产生指标
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ping status = %d, want 200", w.Code)
	}

	// 抓 /metrics,确认三个指标都已暴露且带正确标签
	mw := httptest.NewRecorder()
	r.ServeHTTP(mw, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if mw.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", mw.Code)
	}
	body := mw.Body.String()
	for _, want := range []string{
		`pulsefeed_http_requests_total{method="GET",path="/ping",status="200"} 1`,
		`pulsefeed_http_request_duration_seconds_bucket`,
		`pulsefeed_http_requests_in_flight`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics output missing %q", want)
		}
	}
}
