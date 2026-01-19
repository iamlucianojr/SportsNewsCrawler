package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start MongoDB Container
	mongodbContainer, err := mongodb.Run(ctx, "mongo:6")
	require.NoError(t, err)
	defer func() {
		if err := mongodbContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()

	endpoint, err := mongodbContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// 2. Connect to IT
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(endpoint))
	require.NoError(t, err)
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("failed to disconnect client: %s", err)
		}
	}()

	dbName := "test_news_crawler"
	collectionName := "articles"
	repo, err := repository.NewMongoRepository(client, dbName, collectionName)
	require.NoError(t, err)

	t.Run("Upsert and GetLastFetched", func(t *testing.T) {
		article := &domain.Article{
			ID:          "test-1",
			Source:      "test-source",
			Title:       "Integration Test",
			URL:         "http://test.com/1",
			PublishedAt: time.Now().Truncate(time.Millisecond).UTC(),
			ContentHash: "hash123",
		}

		// Insert
		err := repo.Upsert(ctx, article)
		require.NoError(t, err)

		// Fetch
		fetched, err := repo.GetLastFetched(ctx, "test-source")
		require.NoError(t, err)
		assert.Equal(t, article.ID, fetched.ID)
		assert.Equal(t, article.Title, fetched.Title)
		// Mongo stores times slightly differently, compare roughly or use Truncate if sensitive
		assert.WithinDuration(t, article.PublishedAt, fetched.PublishedAt, time.Millisecond)
	})

	t.Run("BulkUpsert and GetContentHashes", func(t *testing.T) {
		articles := []domain.Article{
			{ID: "b1", Source: "src1", Title: "Batch 1", ContentHash: "h1", PublishedAt: time.Now()},
			{ID: "b2", Source: "src1", Title: "Batch 2", ContentHash: "h2", PublishedAt: time.Now()},
		}

		err := repo.BulkUpsert(ctx, articles)
		require.NoError(t, err)

		hashes, err := repo.GetContentHashes(ctx, []string{"b1", "b2", "non-existent"})
		require.NoError(t, err)
		assert.Equal(t, "h1", hashes["b1"])
		assert.Equal(t, "h2", hashes["b2"])
		assert.Len(t, hashes, 2)
	})
}
