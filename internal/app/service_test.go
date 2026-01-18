package app

import (
	"context"
	"testing"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/stretchr/testify/mock"
)

// Mocks
type MockRepository struct {
	mock.Mock
	articles map[string]domain.Article
}

// Ensure MockRepository implements Repoisitory
var _ domain.Repository = (*MockRepository)(nil)

func (m *MockRepository) Upsert(ctx context.Context, article *domain.Article) error {
	if m.articles == nil {
		m.articles = make(map[string]domain.Article)
	}
	m.articles[article.ID] = *article
	args := m.Called(ctx, article)
	return args.Error(0)
}

func (m *MockRepository) GetLastFetched(ctx context.Context, source string) (*domain.Article, error) {
	args := m.Called(ctx, source)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Article), args.Error(1)
}

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) FetchLatest(ctx context.Context, limit int) ([]domain.Article, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]domain.Article), args.Error(1)
}

func (m *MockProvider) GetName() string {
	return "mock"
}

func (m *MockRepository) BulkUpsert(ctx context.Context, articles []domain.Article) error {
	if m.articles == nil {
		m.articles = make(map[string]domain.Article)
	}
	for _, a := range articles {
		m.articles[a.ID] = a
	}
	args := m.Called(ctx, articles)
	return args.Error(0)
}

func (m *MockRepository) GetContentHashes(ctx context.Context, ids []string) (map[string]string, error) {
	hashes := make(map[string]string)
	if m.articles != nil {
		for _, id := range ids {
			if a, exists := m.articles[id]; exists {
				hashes[id] = a.ContentHash
			}
		}
	}

	args := m.Called(ctx, ids)
	// Return the mocked return values if provided, otherwise use the real map logic?
	// Usually mocks return what we tell them.
	// But here I'm using hybrid. Let's rely on args if present, or just use the map logic for "real" behavior in tests
	// typically mocks just return args.
	// The implementation plan implies using the hash check logic *in the service*.
	// The MockRepository just needs to return what we expect.
	if len(args) > 0 {
		return args.Get(0).(map[string]string), args.Error(1)
	}
	return hashes, nil
}

type MockEventProducer struct {
	mock.Mock
}

func (m *MockEventProducer) Publish(ctx context.Context, article *domain.Article) error {
	args := m.Called(ctx, article)
	return args.Error(0)
}

func (m *MockEventProducer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewsCrawlerService_FetchAndProcess(t *testing.T) {
	repo := new(MockRepository)
	provider := new(MockProvider)
	producer := new(MockEventProducer)

	article := domain.Article{ID: "mock_1", Source: "mock", Title: "Test"}

	provider.On("FetchLatest", mock.Anything, 10).Return([]domain.Article{article}, nil)
	provider.On("GetName").Return("mock").Maybe()

	// Expect GetContentHashes call - return empty map to simulate new article (triggering publish)
	repo.On("GetContentHashes", mock.Anything, []string{"mock_1"}).Return(map[string]string{}, nil)

	// The repo now calls BulkUpsert
	// Note: article passed to BulkUpsert will have ContentHash set now, so we can't match exact 'article' struct unless we set hash on it too.
	// But mock.Anything or matching loosely helps.
	// Or we update article local var to have the hash that generateHash produces.
	// article.ContentHash = "Test|mock|0001-01-01 00:00:00 +0000 UTC" (default string for zero time)
	// Safest is to use mock.Anything for the slice arg or ignore exact match.
	// But let's try to be precise if possible or loose if hard.
	repo.On("BulkUpsert", mock.Anything, mock.Anything).Return(nil)

	// The service now calls Publish
	producer.On("Publish", mock.Anything, mock.Anything).Return(nil)

	// New constructor signature
	svc := NewNewsCrawlerService(repo, []domain.Provider{provider}, producer, 1*time.Second, 10, 5)

	// Since fetchAndProcess is replaced by persistent loops, we test processProvider directly
	// to verify the logic of fetching, deduplicating, UPSERTing, and publishing.
	svc.processProvider(context.Background(), provider)

	provider.AssertExpectations(t)
	repo.AssertExpectations(t)
	producer.AssertExpectations(t)
}
