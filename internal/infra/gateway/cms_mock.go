package gateway

import (
	"context"
	"log/slog"

	"github.com/SportsNewsCrawler/internal/domain"
)

type CMSMockGateway struct{}

func NewCMSMockGateway() *CMSMockGateway {
	return &CMSMockGateway{}
}

func (g *CMSMockGateway) SyncArticle(ctx context.Context, article *domain.Article) error {
	slog.Info("CMS Sync", "article_id", article.ID, "title", article.Title, "source", article.Source, "published_at", article.PublishedAt)
	return nil
}
