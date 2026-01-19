package mocks

import (
	"io"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockTransformer struct {
	mock.Mock
}

func (m *MockTransformer) Transform(reader io.Reader) ([]domain.Article, *domain.PageInfo, error) {
	args := m.Called(reader)

	// Handle nil articles
	var articles []domain.Article
	if args.Get(0) != nil {
		articles = args.Get(0).([]domain.Article)
	}

	// Handle nil pageinfo
	var pageInfo *domain.PageInfo
	if args.Get(1) != nil {
		pageInfo = args.Get(1).(*domain.PageInfo)
	}

	return articles, pageInfo, args.Error(2)
}
