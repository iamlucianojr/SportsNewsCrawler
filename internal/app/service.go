package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type NewsCrawlerService struct {
	repo            domain.Repository
	providers       []domain.Provider
	eventProducer   domain.EventProducer
	interval        time.Duration
	batchSize       int
	workerCount     int
	jobs            chan job
	wg              sync.WaitGroup // Service-wide WaitGroup for graceful shutdown
	activeProviders sync.Map       // Track active provider processing
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

	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// Providers waitgroup tracks PROVIDER goroutines
	var providersWg sync.WaitGroup
	for _, provider := range s.providers {
		slog.Info("Starting provider loop", "provider", provider.GetName())
		providersWg.Add(1)
		go s.runProviderLoop(ctx, provider, &providersWg)
	}

	// Wait for context cancellation
	<-ctx.Done()
	slog.Info("Context cancelled, stopping news crawler service...")

	providersWg.Wait()
	slog.Info("All providers stopped")

	close(s.jobs)

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
		// Prevent concurrent processing of the same provider
		name := j.provider.GetName()
		if _, loaded := s.activeProviders.LoadOrStore(name, true); loaded {
			slog.Warn("Skipping concurrent run", "provider", name, "worker_id", id)
			continue
		}

		metrics.WorkerActiveCount.Inc()
		func() {
			defer s.activeProviders.Delete(name)
			s.processProvider(ctx, j.provider)
		}()
		metrics.WorkerActiveCount.Dec()
	}
	slog.Info("Worker stopped", "worker_id", id)
}

func (s *NewsCrawlerService) processProvider(ctx context.Context, provider domain.Provider) {
	// Start Tracing Span
	tr := otel.Tracer("news-crawler")
	ctx, span := tr.Start(ctx, "processProvider")
	defer span.End()

	slog.Debug("Starting crawl for provider", "provider", provider.GetName())
	span.SetAttributes(attribute.String("provider", provider.GetName()))

	// Define handler that processes each page of articles
	handler := func(articles []domain.Article) error {
		return s.processBatch(ctx, provider, articles)
	}

	if err := provider.Crawl(ctx, handler); err != nil {
		span.RecordError(err)
		slog.Error("Crawl failed", "provider", provider.GetName(), "error", err)
		metrics.ArticlesIngested.WithLabelValues(provider.GetName(), "error_crawl").Inc()
	}
}

func (s *NewsCrawlerService) processBatch(ctx context.Context, provider domain.Provider, articles []domain.Article) error {
	start := time.Now()

	// Dedup within batch
	uniqueArticles := make([]domain.Article, 0, len(articles))
	seenIDs := make(map[string]bool)
	for _, a := range articles {
		if !seenIDs[a.ID] {
			seenIDs[a.ID] = true
			uniqueArticles = append(uniqueArticles, a)
		}
	}
	articles = uniqueArticles

	if len(articles) == 0 {
		return nil
	}

	// 1. Calculate Hashes
	var ids []string
	for i := range articles {
		articles[i].ContentHash = generateHash(&articles[i])
		ids = append(ids, articles[i].ID)
	}

	// 2. Fetch Existing Hashes
	existingHashes, err := s.repo.GetContentHashes(ctx, ids)
	if err != nil {
		return fmt.Errorf("failed to fetch hashes: %w", err)
	}

	// 3. Identify Changed Articles
	var changedArticles []domain.Article
	skippedCount := 0
	for _, article := range articles {
		oldHash, exists := existingHashes[article.ID]
		if !exists {
			slog.Info("Article New", "provider", provider.GetName(), "id", article.ID)
			changedArticles = append(changedArticles, article)
		} else if oldHash != article.ContentHash {
			slog.Info("Article Changed", "provider", provider.GetName(), "id", article.ID)
			changedArticles = append(changedArticles, article)
		} else {
			skippedCount++
		}
	}

	if skippedCount > 0 {
		metrics.ArticlesDuplicatesSkipped.WithLabelValues(provider.GetName()).Add(float64(skippedCount))
	}

	metrics.ProviderFetchDuration.WithLabelValues(provider.GetName()).Observe(time.Since(start).Seconds())
	metrics.ArticlesIngested.WithLabelValues(provider.GetName(), "success").Add(float64(len(articles)))
	// Initialize published metrics to ensure they appear in Grafana even if 0
	metrics.ArticlesPublished.WithLabelValues(provider.GetName()).Add(0)
	metrics.PublishErrors.WithLabelValues(provider.GetName()).Add(0)

	// Track freshness
	for _, a := range articles {
		if !a.PublishedAt.IsZero() {
			metrics.ArticleFreshness.WithLabelValues(provider.GetName()).Observe(time.Since(a.PublishedAt).Seconds())
		}
	}

	// 4. Bulk Upsert
	if err := s.repo.BulkUpsert(ctx, articles); err != nil {
		return fmt.Errorf("bulk upsert failed: %w", err)
	}

	// 5. Publish Changed
	if len(changedArticles) > 0 {
		slog.Info("Publishing changed articles", "count", len(changedArticles), "provider", provider.GetName())

		pubStart := time.Now()
		err := s.eventProducer.PublishBatch(ctx, changedArticles)
		metrics.PublishDuration.WithLabelValues(provider.GetName()).Observe(time.Since(pubStart).Seconds())

		if err != nil {
			slog.Error("Error publishing article batch", "count", len(changedArticles), "error", err)
			metrics.PublishErrors.WithLabelValues(provider.GetName()).Inc()
			// Continue even if publish fails, data is in DB
		} else {
			metrics.ArticlesPublished.WithLabelValues(provider.GetName()).Add(float64(len(changedArticles)))
		}
	}

	return nil
}

func generateHash(a *domain.Article) string {
	// Use SHA256 of content fields to detect changes
	// We include Title, Summary, Body, Source, and URL.
	// We intentionally exclude PublishedAt/FetchedAt to detect "Update Same Content" vs "New Content"
	hasher := sha256.New()
	hasher.Write([]byte(a.Source))
	hasher.Write([]byte(a.URL))
	hasher.Write([]byte(a.Title))
	hasher.Write([]byte(a.Summary))
	hasher.Write([]byte(a.Body))
	return hex.EncodeToString(hasher.Sum(nil))
}
