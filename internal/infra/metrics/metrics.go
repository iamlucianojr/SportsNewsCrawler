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

	IngestionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ingestion_duration_seconds",
			Help:    "Duration of ingestion cycles",
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
)
