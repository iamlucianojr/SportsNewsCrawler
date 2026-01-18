package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type NewsCrawlerService struct {
	repo          domain.Repository
	providers     []domain.Provider
	eventProducer domain.EventProducer
	interval      time.Duration
	batchSize     int
	workerCount   int
	jobs          chan job
	wg            sync.WaitGroup // Service-wide WaitGroup for graceful shutdown
}

type job struct {
	provider domain.Provider
}

func NewNewsCrawlerService(
	repo domain.Repository,
	providers []domain.Provider,
	eventProducer domain.EventProducer,
	interval time.Duration,
	batchSize int,
	workerCount int,
) *NewsCrawlerService {
	return &NewsCrawlerService{
		repo:          repo,
		providers:     providers,
		eventProducer: eventProducer,
		interval:      interval,
		batchSize:     batchSize,
		workerCount:   workerCount,
		jobs:          make(chan job, workerCount*2), // Buffer to avoid blocking providers immediately
	}
}

func (s *NewsCrawlerService) Start(ctx context.Context) {
	slog.Info("Starting news crawler service", "interval", s.interval, "workers", s.workerCount)

	// Service-wide waitgroup tracks WORKER goroutines
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// Providers waitgroup tracks PROVIDER goroutines
	var providersWg sync.WaitGroup
	for _, provider := range s.providers {
		providersWg.Add(1)
		go s.runProviderLoop(ctx, provider, &providersWg)
	}

	// Wait for context cancellation
	<-ctx.Done()
	slog.Info("Context cancelled, stopping news crawler service...")

	// 1. Wait for all providers to stop producing jobs
	providersWg.Wait()
	slog.Info("All providers stopped")

	// 2. Close jobs channel to signal workers to drain and exit
	close(s.jobs)

	// 3. Wait for all workers to finish processing
	s.wg.Wait()
	slog.Info("All workers stopped")
}

func (s *NewsCrawlerService) runProviderLoop(ctx context.Context, p domain.Provider, wg *sync.WaitGroup) {
	defer wg.Done()

	// Initial fetch
	select {
	case s.jobs <- job{provider: p}:
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			// Non-blocking push if channel full? Or blocking?
			// Blocking is safer for backpressure, but if workers are stuck, all providers stick.
			// Given buffer, blocking is fine.
			case s.jobs <- job{provider: p}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *NewsCrawlerService) worker(ctx context.Context, id int) {
	defer s.wg.Done()
	slog.Info("Worker started", "worker_id", id)

	// Range over channel handles closing correctly: exits loop when closed and empty
	for j := range s.jobs {
		metrics.WorkerActiveCount.Inc()
		s.processProvider(ctx, j.provider)
		metrics.WorkerActiveCount.Dec()
	}
	slog.Info("Worker stopped", "worker_id", id)
}

func (s *NewsCrawlerService) processProvider(ctx context.Context, provider domain.Provider) {
	// Start Tracing Span
	tr := otel.Tracer("news-crawler")
	ctx, span := tr.Start(ctx, "processProvider")
	defer span.End()

	start := time.Now()
	span.SetAttributes(attribute.String("provider", provider.GetName()))
	slog.Debug("Polling provider", "provider", provider.GetName())

	articles, err := provider.FetchLatest(ctx, s.batchSize)
	if err != nil {
		span.RecordError(err)
		slog.Error("Error fetching from provider", "provider", provider.GetName(), "error", err)
		metrics.ArticlesIngested.WithLabelValues(provider.GetName(), "error_fetch").Inc()
		return
	}
	span.SetAttributes(attribute.Int("articles_fetched", len(articles)))

	if len(articles) == 0 {
		return
	}

	// 1. Calculate Hashes
	var ids []string
	for i := range articles {
		// Simple hash generation (Title + PublishedAt) - in production use a stronger hash of Content
		articles[i].ContentHash = generateHash(&articles[i])
		ids = append(ids, articles[i].ID)
	}

	// 2. Fetch Existing Hashes
	existingHashes, err := s.repo.GetContentHashes(ctx, ids)
	if err != nil {
		slog.Error("Failed to fetch existing hashes", "error", err)
		// Proceed? Or fail? Fail safe to avoid spam
		return
	}

	// 3. Identify Changed Articles
	var changedArticles []domain.Article
	skippedCount := 0
	for _, article := range articles {
		oldHash, exists := existingHashes[article.ID]
		if !exists || oldHash != article.ContentHash {
			changedArticles = append(changedArticles, article)
		} else {
			skippedCount++
		}
	}

	if skippedCount > 0 {
		metrics.ArticlesDuplicatesSkipped.WithLabelValues(provider.GetName()).Add(float64(skippedCount))
	}

	metrics.IngestionDuration.WithLabelValues(provider.GetName()).Observe(time.Since(start).Seconds())
	metrics.ArticlesIngested.WithLabelValues(provider.GetName(), "success").Add(float64(len(articles)))

	// 4. Bulk Upsert (Update all, including timestamps)
	if err := s.repo.BulkUpsert(ctx, articles); err != nil {
		slog.Error("Error bulk inserting articles", "error", err)
		return
	}

	// 5. Publish ONLY changed articles
	if len(changedArticles) > 0 {
		slog.Info("Publishing changed articles", "count", len(changedArticles), "provider", provider.GetName())
		for _, article := range changedArticles {
			if err := s.eventProducer.Publish(ctx, &article); err != nil {
				slog.Error("Error publishing article event", "article_id", article.ID, "error", err)
				continue
			}
		}
	}
}

func generateHash(a *domain.Article) string {
	// Use SHA256 of content fields to detect changes
	// We include Title, Summary, Body, and Source.
	// We intentionally exclude PublishedAt/FetchedAt to detect "Update Same Content" vs "New Content"
	// However, if the source changes the *timestamp* but not content, we usually don't want to re-process unless needed.
	// But if we exclude timestamp, different articles with same title/body (rare) might collide? Unlikely for news.
	// Let's include what defines "Content Identity".
	hasher := sha256.New()
	hasher.Write([]byte(a.Source))
	hasher.Write([]byte(a.Title))
	hasher.Write([]byte(a.Summary))
	hasher.Write([]byte(a.Body))
	return hex.EncodeToString(hasher.Sum(nil))
}
