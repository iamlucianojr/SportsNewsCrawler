package app

import (
	"context"
	"testing"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks
type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) GetContentHashes(ctx context.Context, ids []string) (map[string]string, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockRepo) BulkUpsert(ctx context.Context, articles []domain.Article) error {
	args := m.Called(ctx, articles)
	return args.Error(0)
}

func (m *MockRepo) GetLastFetched(ctx context.Context, source string) (*domain.Article, error) {
	return nil, nil
}
func (m *MockRepo) Upsert(ctx context.Context, article *domain.Article) error { return nil }


type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) PublishBatch(ctx context.Context, articles []domain.Article) error {
	args := m.Called(ctx, articles)
	return args.Error(0)
}
func (m *MockProducer) Publish(ctx context.Context, article *domain.Article) error { return nil }
func (m *MockProducer) Close() error { return nil }

type MockProvider struct {
	mock.Mock
}
func (m *MockProvider) Crawl(ctx context.Context, handler func([]domain.Article) error) error {
	// Not used in unit test of processBatch
	return nil 
}
func (m *MockProvider) GetName() string { return "test-provider" }

// Test Service logic
func TestNewsCrawlerService_Deduplication(t *testing.T) {
	repo := new(MockRepo)
	producer := new(MockProducer)
	provider := new(MockProvider)

	service := NewNewsCrawlerService(repo, []domain.Provider{provider}, producer, time.Minute, 10, 1)

	// Article 1: New
	// Article 2: Changed
	// Article 3: Unchanged
	
	a1 := domain.Article{ID: "1", Title: "New", Source: "test", URL: "u1"}
	a2 := domain.Article{ID: "2", Title: "Changed", Source: "test", URL: "u2"}
	a3 := domain.Article{ID: "3", Title: "Same", Source: "test", URL: "u3"}

	a1.ContentHash = a1.ComputeHash()
	a2.ContentHash = a2.ComputeHash()
	a3.ContentHash = a3.ComputeHash()

	existingHashes := map[string]string{
		"2": "old_hash",
		"3": a3.ContentHash, // Matches current
	}

	repo.On("GetContentHashes", mock.Anything, []string{"1", "2", "3"}).Return(existingHashes, nil)
	repo.On("BulkUpsert", mock.Anything, mock.MatchedBy(func(articles []domain.Article) bool {
		return len(articles) == 3 // Should upsert all
	})).Return(nil)
	
	// Expect only 1 and 2 to be published
	producer.On("PublishBatch", mock.Anything, mock.MatchedBy(func(articles []domain.Article) bool {
		if len(articles) != 2 { return false }
		ids := map[string]bool{articles[0].ID: true, articles[1].ID: true}
		return ids["1"] && ids["2"] && !ids["3"]
	})).Return(nil)

	err := service.processBatch(context.Background(), provider, []domain.Article{a1, a2, a3})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	producer.AssertExpectations(t)
}