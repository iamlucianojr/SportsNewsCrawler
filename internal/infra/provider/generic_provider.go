package provider

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/sony/gobreaker"
)

type GenericProvider struct {
	name        string
	url         string
	client      *http.Client
	transformer domain.Transformer
	cb          *gobreaker.CircuitBreaker
}

func NewGenericProvider(name, url string, transformer domain.Transformer) *GenericProvider {
	cbSettings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip if we have 3 consecutive failures
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Warn("CircuitBreaker state changed", "name", name, "from", from, "to", to)
		},
	}

	return &GenericProvider{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		transformer: transformer,
		cb:          gobreaker.NewCircuitBreaker(cbSettings),
	}
}

func (p *GenericProvider) GetName() string {
	return p.name
}

func (p *GenericProvider) FetchLatest(ctx context.Context, limit int) ([]domain.Article, error) {
	var allArticles []domain.Article
	page := 0
	maxPages := 10 // Safety limit to prevent infinite loops

	slog.Debug("Starting paginated fetch", "provider", p.name, "limit", limit)

	for page < maxPages && len(allArticles) < limit {
		// Build URL with page parameter
		pageURL := p.buildURLWithPage(page)

		// Fetch single page
		articles, hasMore, err := p.fetchSinglePage(ctx, pageURL, page)
		if err != nil {
			// Log error but don't fail completely if we have some articles
			if len(allArticles) > 0 {
				slog.Warn("Error fetching page, returning partial results",
					"provider", p.name, "page", page, "error", err, "articles_collected", len(allArticles))
				break
			}
			return nil, err
		}

		if len(articles) == 0 {
			slog.Debug("No articles on page, stopping", "provider", p.name, "page", page)
			break
		}

		allArticles = append(allArticles, articles...)

		slog.Info("Fetched page",
			"provider", p.name,
			"page", page,
			"articles_on_page", len(articles),
			"total_articles", len(allArticles))

		// Stop if API indicates no more pages
		if !hasMore {
			slog.Debug("No more pages available", "provider", p.name, "page", page)
			break
		}

		page++
	}

	if page >= maxPages {
		slog.Warn("Reached max pages limit", "provider", p.name, "max_pages", maxPages)
	}

	// Apply limit
	if len(allArticles) > limit {
		slog.Debug("Applying limit", "provider", p.name, "fetched", len(allArticles), "limit", limit)
		return allArticles[:limit], nil
	}

	return allArticles, nil
}

func (p *GenericProvider) buildURLWithPage(page int) string {
	// If URL already has query params, add page parameter
	if page == 0 {
		return p.url // Use original URL for first page
	}

	// Add page parameter
	separator := "?"
	if len(p.url) > 0 && (p.url[len(p.url)-1:] == "?" || contains(p.url, "?")) {
		separator = "&"
	}

	return fmt.Sprintf("%s%spage=%d", p.url, separator, page)
}

func (p *GenericProvider) fetchSinglePage(ctx context.Context, url string, page int) ([]domain.Article, bool, error) {
	var body io.ReadCloser
	var err error

	// Retry configuration
	maxRetries := 3
	backoff := 500 * time.Millisecond

	// Execute with Circuit Breaker and Retries
	_, err = p.cb.Execute(func() (interface{}, error) {
		for i := 0; i <= maxRetries; i++ {
			if i > 0 {
				slog.Info("Retrying request", "provider", p.name, "page", page, "attempt", i, "max_retries", maxRetries)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					backoff *= 2 // Exponential backoff
				}
			}

			// Actual HTTP Request
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if reqErr != nil {
				return nil, fmt.Errorf("failed to create request: %w", reqErr)
			}

			resp, respErr := p.client.Do(req)
			if respErr != nil {
				slog.Warn("Request failed", "provider", p.name, "page", page, "error", respErr)
				continue // Retry on network error
			}

			// Check status code
			if resp.StatusCode >= 500 {
				if err := resp.Body.Close(); err != nil {
					slog.Warn("Failed to close response body", "error", err)
				}
				slog.Warn("Server error", "provider", p.name, "page", page, "status_code", resp.StatusCode)
				continue // Retry on 5xx
			}

			if resp.StatusCode != http.StatusOK {
				if err := resp.Body.Close(); err != nil {
					slog.Warn("Failed to close response body", "error", err)
				}
				// Don't retry on 4xx (client error), just fail
				return nil, fmt.Errorf("provider %s returned status %d", p.name, resp.StatusCode)
			}

			// Success
			body = resp.Body
			return nil, nil
		}
		return nil, fmt.Errorf("max retries exceeded")
	})

	if err != nil {
		return nil, false, fmt.Errorf("circuit breaker execute failed: %w", err)
	}
	defer func() {
		if err := body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}()

	// Transform
	articles, err := p.transformer.Transform(body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to transform articles from %s: %w", p.name, err)
	}

	// Assume more pages exist if we got a full page
	// This is a heuristic; ideally we'd parse pageInfo from response
	hasMore := len(articles) > 0

	return articles, hasMore, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		(len(s) > len(substr) && s[1:len(substr)+1] == substr)
}
