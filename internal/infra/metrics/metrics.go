package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ArticlesIngested = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "articles_ingested_total",
			Help: "The total number of articles ingested",
		},
		[]string{"source", "status"},
	)

	ArticlesDuplicatesSkipped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "articles_duplicates_skipped_total",
			Help: "The total number of articles skipped because they already exist",
		},
		[]string{"source"},
	)

	ProviderFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "provider_fetch_duration_seconds",
			Help:    "Duration of fetch cycles from providers",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"},
	)

	WorkerActiveCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_active_count",
			Help: "Number of workers currently processing jobs",
		},
	)

	ArticlesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "articles_published_total",
			Help: "Total number of articles successfully published to Kafka",
		},
		[]string{"source"},
	)

	PublishDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "publish_duration_seconds",
			Help:    "Duration of publishing to Kafka",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"},
	)

	PublishErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "publish_errors_total",
			Help: "Total number of errors while publishing to Kafka",
		},
		[]string{"source"},
	)

	DLQMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dlq_messages_published_total",
			Help: "Total number of messages published to DLQ",
		},
		[]string{"source"},
	)

	CMSSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "cms_sync_duration_seconds",
			Help:    "Duration of CMS synchronization",
			Buckets: prometheus.DefBuckets,
		},
	)

	CMSSyncErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cms_sync_errors_total",
			Help: "Total number of CMS sync errors",
		},
		[]string{"source"},
	)

	CMSSyncSuccess = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cms_articles_processed_total",
			Help: "Total number of articles successfully synced to CMS",
		},
		[]string{"source"},
	)
	ArticleFreshness = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "article_freshness_seconds",
			Help:    "Time since article publication when ingested",
			Buckets: []float64{60, 300, 600, 1800, 3600, 7200, 21600, 86400}, // 1m, 5m, 10m, 30m, 1h, 2h, 6h, 24h
		},
		[]string{"source"},
	)

	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "State of the circuit breaker (0=Closed, 1=Half-Open, 2=Open)",
		},
		[]string{"source"},
	)

	ParseErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "article_parse_errors_total",
			Help: "Total number of errors during article parsing/transformation",
		},
		[]string{"source"},
	)
)
