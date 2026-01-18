package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/metrics"
	"github.com/SportsNewsCrawler/internal/infra/queue"
)

type CMSSyncService struct {
	consumer   *queue.KafkaConsumer
	cmsGateway domain.CMSGateway
}

func NewCMSSyncService(consumer *queue.KafkaConsumer, cmsGateway domain.CMSGateway) *CMSSyncService {
	return &CMSSyncService{
		consumer:   consumer,
		cmsGateway: cmsGateway,
	}
}

func (s *CMSSyncService) Start(ctx context.Context) {
	slog.Info("Starting CMS Sync Service (Kafka Consumer)")
	go s.consumer.Start(ctx, s.handleEvent)
}

func (s *CMSSyncService) handleEvent(ctx context.Context, article *domain.Article) error {
	start := time.Now()
	// Here we receive the article from Kafka and sync it to the CMS
	slog.Info("Consuming event for sync", "article_id", article.ID, "title", article.Title)

	err := s.cmsGateway.SyncArticle(ctx, article)
	duration := time.Since(start).Seconds()
	metrics.CMSSyncDuration.Observe(duration)

	if err != nil {
		slog.Error("Failed to sync article to CMS", "article_id", article.ID, "error", err)
		metrics.CMSSyncErrors.WithLabelValues(article.Source).Inc()
		return err
	}

	metrics.CMSSyncSuccess.WithLabelValues(article.Source).Inc()
	return nil
}

func (s *CMSSyncService) Stop() error {
	return s.consumer.Close()
}
