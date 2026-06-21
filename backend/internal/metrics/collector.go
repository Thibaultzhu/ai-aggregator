package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// Collector wraps Prometheus metrics for the aggregator.
type Collector struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	ttftDuration     *prometheus.HistogramVec
	tokensTotal      *prometheus.CounterVec
	costUSD          *prometheus.CounterVec
	asyncTasks       *prometheus.GaugeVec
	rateLimitReject  *prometheus.CounterVec
	activeStreams    *prometheus.GaugeVec
}

func NewCollector() *Collector {
	return &Collector{
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aggregator",
			Name:      "requests_total",
			Help:      "Total number of API requests",
		}, []string{"model", "provider", "status", "modality", "stream"}),

		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "aggregator",
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.05, 2, 14), // 50ms ~ 819.2s
		}, []string{"model", "provider", "modality"}),

		ttftDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "aggregator",
			Name:      "ttft_seconds",
			Help:      "Time to first token for streaming requests",
			Buckets:   prometheus.ExponentialBuckets(0.05, 2, 10), // 50ms ~ 51.2s
		}, []string{"model", "provider"}),

		tokensTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aggregator",
			Name:      "tokens_total",
			Help:      "Total tokens processed",
		}, []string{"model", "direction"}), // direction: input/output

		costUSD: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aggregator",
			Name:      "cost_usd_total",
			Help:      "Total cost in USD",
		}, []string{"model", "provider", "type"}), // type: upstream/charged

		asyncTasks: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "aggregator",
			Name:      "async_tasks_inflight",
			Help:      "Number of async tasks currently being processed",
		}, []string{"model"}),

		rateLimitReject: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "aggregator",
			Name:      "rate_limit_rejections_total",
			Help:      "Total rate limit rejections",
		}, []string{"key_id", "type"}),

		activeStreams: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "aggregator",
			Name:      "active_streams",
			Help:      "Number of active streaming connections",
		}, []string{"model"}),
	}
}

// RecordRequest records a completed request.
func (c *Collector) RecordRequest(model, provider, modality string, isStream bool, statusCode int, duration, ttft time.Duration) {
	c.requestsTotal.WithLabelValues(model, provider, strconv.Itoa(statusCode), modality, strconv.FormatBool(isStream)).Inc()
	c.requestDuration.WithLabelValues(model, provider, modality).Observe(duration.Seconds())
	if isStream && ttft > 0 {
		c.ttftDuration.WithLabelValues(model, provider).Observe(ttft.Seconds())
	}
}

// RecordTokens records token usage.
func (c *Collector) RecordTokens(model, direction string, count int) {
	if count > 0 {
		c.tokensTotal.WithLabelValues(model, direction).Add(float64(count))
	}
}

// RecordCost records cost.
func (c *Collector) RecordCost(model, provider, costType string, amount float64) {
	c.costUSD.WithLabelValues(model, provider, costType).Add(amount)
}

// IncrAsyncTask increments the async task gauge.
func (c *Collector) IncrAsyncTask(model string) {
	c.asyncTasks.WithLabelValues(model).Inc()
}

// DecrAsyncTask decrements the async task gauge.
func (c *Collector) DecrAsyncTask(model string) {
	c.asyncTasks.WithLabelValues(model).Dec()
}

// Handler returns the Prometheus metrics HTTP handler for Fiber.
func (c *Collector) Handler(ctx *fiber.Ctx) error {
	h := promhttp.Handler()
	fasthttpadaptor.NewFastHTTPHandler(h)(ctx.Context())
	return nil
}
