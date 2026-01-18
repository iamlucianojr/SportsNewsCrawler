package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	db         *mongo.Database
	collection *mongo.Collection
}

func NewMongoRepository(client *mongo.Client, dbName, collectionName string) (*MongoRepository, error) {
	db := client.Database(dbName)
	repo := &MongoRepository{
		db:         db,
		collection: db.Collection(collectionName),
	}

	if err := repo.createIndexes(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return repo, nil
}

func (r *MongoRepository) createIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "source", Value: 1},
				{Key: "published_at", Value: -1},
			},
			Options: options.Index().SetName("source_published_at_idx"),
		},
		{
			Keys: bson.D{
				{Key: "external_id", Value: 1},
			},
			Options: options.Index().SetName("external_id_idx"),
		},
	}

	opts := options.CreateIndexes().SetMaxTime(10 * time.Second)
	_, err := r.collection.Indexes().CreateMany(ctx, models, opts)
	return err
}

func (r *MongoRepository) Upsert(ctx context.Context, article *domain.Article) error {
	filter := bson.M{"_id": article.ID}
	update := bson.M{"$set": article}
	opts := options.Update().SetUpsert(true)

	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert article: %w", err)
	}
	return nil
}

func (r *MongoRepository) BulkUpsert(ctx context.Context, articles []domain.Article) error {
	if len(articles) == 0 {
		return nil
	}

	var models []mongo.WriteModel
	for _, article := range articles {
		filter := bson.M{"_id": article.ID}
		update := bson.M{"$set": article}
		model := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
		models = append(models, model)
	}

	opts := options.BulkWrite().SetOrdered(false)
	_, err := r.collection.BulkWrite(ctx, models, opts)
	if err != nil {
		return fmt.Errorf("failed to bulk upsert articles: %w", err)
	}
	return nil
}

func (r *MongoRepository) GetLastFetched(ctx context.Context, source string) (*domain.Article, error) {
	filter := bson.M{"source": source}
	opts := options.FindOne().SetSort(bson.D{{Key: "published_at", Value: -1}})

	var article domain.Article
	err := r.collection.FindOne(ctx, filter, opts).Decode(&article)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &article, err
}

func (r *MongoRepository) GetContentHashes(ctx context.Context, ids []string) (map[string]string, error) {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	opts := options.Find()
	// Only fetch _id and content_hash
	opts.SetProjection(bson.M{"_id": 1, "content_hash": 1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			slog.Warn("Failed to close cursor", "error", err)
		}
	}()

	results := make(map[string]string)
	for cursor.Next(ctx) {
		var doc struct {
			ID          string `bson:"_id"`
			ContentHash string `bson:"content_hash"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue // Skip malformed
		}
		results[doc.ID] = doc.ContentHash
	}
	return results, cursor.Err()
}
