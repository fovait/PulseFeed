package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequestsTotal 累计请求数,按 方法/路由/状态码 分维度,可据此算 QPS 与错误率。
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulsefeed_http_requests_total",
			Help: "Total number of HTTP requests by method, path and status code.",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration 请求耗时直方图,用于算 p50/p95/p99 延迟分位。
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulsefeed_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds by method, path and status code.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestsInFlight 当前在途请求数,反映瞬时并发与堆积。
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulsefeed_http_requests_in_flight",
			Help: "Number of HTTP requests currently being served.",
		},
	)
)

// MetricsMiddleware 采集每个请求的 QPS / 延迟 / 错误率 / 在途数。
// 路由维度用 c.FullPath()(路由模板,而非真实 URL),避免把路径参数打成高基数标签。
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// /metrics 自身不计入,避免抓取流量污染业务指标。
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched" // 未命中任何路由(404 等),归一化避免空标签
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())
	}
}

// MetricsHandler 暴露 Prometheus 抓取端点(默认注册表)。
func MetricsHandler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}
