package provider

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/metrics"
	"github.com/SportsNewsCrawler/pkg/config"
	"github.com/sony/gobreaker"
)

type GenericProvider struct {
	name        string
	url         string
	client      *http.Client
	transformer domain.Transformer
	pagination  config.PaginationConfig
	cb          *gobreaker.CircuitBreaker
}

func NewGenericProvider(name, url string, transformer domain.Transformer, pagination config.PaginationConfig) *GenericProvider {
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
			var stateVal float64
			switch to {
			case gobreaker.StateClosed:
				stateVal = 0
			case gobreaker.StateHalfOpen:
				stateVal = 1
			case gobreaker.StateOpen:
				stateVal = 2
			}
			// Update global metrics directly to maintain constructor signature
			metrics.CircuitBreakerState.WithLabelValues(name).Set(stateVal)
		},
	}

	return &GenericProvider{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		transformer: transformer,
		pagination:  pagination,
		cb:          gobreaker.NewCircuitBreaker(cbSettings),
	}
}

func (p *GenericProvider) GetName() string {
	return p.name
}

const maxSafetyPages = 1000

func (p *GenericProvider) Crawl(ctx context.Context, handler func([]domain.Article) error) error {
	slog.Debug("Starting streaming crawl", "provider", p.name)

	if err := p.crawlLoop(ctx, handler); err != nil {
		return err
	}

	return nil
}

func (p *GenericProvider) crawlLoop(ctx context.Context, handler func([]domain.Article) error) error {
	page := 0
	numPages := -1 // Unknown initially
	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

	for page < maxSafetyPages {
		// Stop if we know the total pages and have reached it
		if numPages != -1 && page >= numPages {
			slog.Debug("Reached total pages", "provider", p.name, "page", page, "total_pages", numPages)
			break
		}

		// Build URL with page parameter
		pageURL := p.buildURLWithPage(page)

		// Fetch single page
		articles, pageInfo, err := p.fetchSinglePage(ctx, pageURL, page)
		if err != nil {
			slog.Error("Error fetching page, stopping crawl", "provider", p.name, "page", page, "error", err)
			return err
		}

		if len(articles) == 0 {
			slog.Debug("No articles on page, stopping", "provider", p.name, "page", page)
			break
		}

		// Process batch immediately via handler
		if err := handler(articles); err != nil {
			slog.Error("Handler failed (continuing)", "provider", p.name, "page", page, "error", err)
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				return fmt.Errorf("too many consecutive handler errors (%d): %w", consecutiveErrors, err)
			}
		} else {
			consecutiveErrors = 0
			slog.Info("Processed page",
				"provider", p.name,
				"page", page,
				"articles_count", len(articles))
		}

		// Update numPages from metadata if available
		// PageInfo tracks the state of pagination from the transformer
		if pageInfo != nil {
			numPages = pageInfo.NumPages
		}

		page++
	}

	if page >= maxSafetyPages {
		slog.Warn("Reached max safety pages limit", "provider", p.name, "max_pages", maxSafetyPages)
	}

	return nil
}

func (p *GenericProvider) buildURLWithPage(page int) string {
	reqURL := p.url
	separator := "?"
	if len(reqURL) > 0 && (reqURL[len(reqURL)-1:] == "?" || contains(reqURL, "?")) {
		separator = "&"
	}

	pageParam := "page"
	if p.pagination.PageParam != "" {
		pageParam = p.pagination.PageParam
	}

	limitParam := "pageSize"
	if p.pagination.LimitParam != "" {
		limitParam = p.pagination.LimitParam
	}

	defaultLimit := 20
	if p.pagination.DefaultLimit > 0 {
		defaultLimit = p.pagination.DefaultLimit
	}

	var params string
	if p.pagination.Type == "offset" {
		offset := page * defaultLimit
		params = fmt.Sprintf("%s=%d&%s=%d", pageParam, offset, limitParam, defaultLimit)
	} else {
		// Use 0-indexed pagination
		params = fmt.Sprintf("%s=%d&%s=%d", pageParam, page, limitParam, defaultLimit)
	}

	return fmt.Sprintf("%s%s%s", reqURL, separator, params)
}

func (p *GenericProvider) fetchSinglePage(ctx context.Context, url string, page int) ([]domain.Article, *domain.PageInfo, error) {
	// Execute Request with Retries and Circuit Breaker
	body, err := p.executeRequest(ctx, url, page)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}()

	// Transform
	articles, pageInfo, err := p.transformer.Transform(body)
	if err != nil {
		// Record parse error
		metrics.ParseErrors.WithLabelValues(p.name).Inc()
		return nil, nil, fmt.Errorf("failed to transform articles from %s: %w", p.name, err)
	}

	return articles, pageInfo, nil
}

func (p *GenericProvider) executeRequest(ctx context.Context, url string, page int) (io.ReadCloser, error) {
	maxRetries := 3
	backoff := 500 * time.Millisecond

	val, err := p.cb.Execute(func() (interface{}, error) {
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
			slog.Info("Fetching URL", "url", url)
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
				// Fail immediately on client errors
				return nil, fmt.Errorf("provider %s returned status %d", p.name, resp.StatusCode)
			}

			// Return response body for caller to close
			return resp.Body, nil
		}
		return nil, fmt.Errorf("max retries exceeded")
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker execute failed: %w", err)
	}

	return val.(io.ReadCloser), nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
