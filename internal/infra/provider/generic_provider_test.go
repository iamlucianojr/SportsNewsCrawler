package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTransformer is a mock implementation of domain.Transformer
type MockTransformer struct {
	mock.Mock
}

func (m *MockTransformer) Transform(reader io.Reader) ([]domain.Article, *domain.PageInfo, error) {
	args := m.Called(reader)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]domain.Article), args.Get(1).(*domain.PageInfo), args.Error(2)
}

func TestGenericProvider_Crawl_Resilience(t *testing.T) {
	// Setup Mock Server
	// Page 0: Returns 1 article
	// Page 1: Returns 1 article
	// Page 2: Returns 1 article
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) // Content irrelevant as transformer is mocked
	}))
	defer server.Close()

	mockTransformer := new(MockTransformer)
	
	// Mock 3 successful transformations for 3 pages
	mockTransformer.On("Transform", mock.Anything).Return(
		[]domain.Article{{ID: "1"}}, 
		&domain.PageInfo{Page: 0, NumPages: 3, PageSize: 1, NumEntries: 3}, 
		nil,
	).Once()
	
	mockTransformer.On("Transform", mock.Anything).Return(
		[]domain.Article{{ID: "2"}}, 
		&domain.PageInfo{Page: 1, NumPages: 3, PageSize: 1, NumEntries: 3}, 
		nil,
	).Once()

	mockTransformer.On("Transform", mock.Anything).Return(
		[]domain.Article{{ID: "3"}}, 
		&domain.PageInfo{Page: 2, NumPages: 3, PageSize: 1, NumEntries: 3}, 
		nil,
	).Once()

	provider := NewGenericProvider(
		"test-provider",
		server.URL,
		mockTransformer,
		config.PaginationConfig{
			Type: "page", 
			PageParam: "page",
			LimitParam: "pageSize",
			DefaultLimit: 10,
		},
	)

	// Test Scenario: Handler fails for the 2nd page (Article ID "2"), but crawl should continue
	processedIDs := []string{}
	err := provider.Crawl(context.Background(), func(articles []domain.Article) error {
		if articles[0].ID == "2" {
			return errors.New("simulated database error")
		}
		processedIDs = append(processedIDs, articles[0].ID)
		return nil
	})

	assert.NoError(t, err, "Crawl should not return error for a single batch failure")
	assert.Equal(t, []string{"1", "3"}, processedIDs, "Should have processed page 0 and 2, skipping page 1")
	
	mockTransformer.AssertExpectations(t)
}

func TestGenericProvider_Crawl_CircuitBreaker_Abort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	mockTransformer := new(MockTransformer)
	// Return success for transform, but handler will fail repeatedly
	mockTransformer.On("Transform", mock.Anything).Return(
		[]domain.Article{{ID: "x"}}, 
		&domain.PageInfo{NumPages: 10}, 
		nil,
	)

	provider := NewGenericProvider("test-provider", server.URL, mockTransformer, config.PaginationConfig{})

	// Fail every time
	consecutiveFailures := 0
	err := provider.Crawl(context.Background(), func(articles []domain.Article) error {
		consecutiveFailures++
		return errors.New("persistent error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many consecutive handler errors")
	assert.Equal(t, 5, consecutiveFailures, "Should stop after 5 failures")
}
