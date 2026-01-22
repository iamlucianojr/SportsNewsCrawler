package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Article represents the internal normalized sports news article format.
type Article struct {
	ID          string    `json:"id" bson:"_id"`
	Source      string    `json:"source" bson:"source"` // e.g., "pulselive"
	ExternalID  string    `json:"external_id" bson:"external_id"`
	Type        string    `json:"type" bson:"type"` // Content type: "text", "video", etc.
	Title       string    `json:"title" bson:"title"`
	Description string    `json:"description" bson:"description"` // Short description (better than summary)
	Summary     string    `json:"summary" bson:"summary"`
	Body        string    `json:"body" bson:"body"`
	Content     string    `json:"content" bson:"content"` // Added content if needed, or alias Body
	URL         string    `json:"url" bson:"url"`
	ImageURL    string    `json:"image_url" bson:"image_url"`
	Tags        []Tag     `json:"tags" bson:"tags"` // Content tags for categorization
	PublishedAt time.Time `json:"published_at" bson:"published_at"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
	FetchedAt   time.Time `json:"fetched_at" bson:"fetched_at"`
	ContentHash string    `json:"content_hash" bson:"content_hash"` // New field for deduplication
}

// ComputeHash generates a deterministic hash of the article's content.
// It includes Source, URL, Title, Summary, and Body to detect "New Content" vs "Update Same Content".
func (a *Article) ComputeHash() string {
	hasher := sha256.New()
	hasher.Write([]byte(a.Source))
	hasher.Write([]byte(a.URL))
	hasher.Write([]byte(a.Title))
	hasher.Write([]byte(a.Summary))
	hasher.Write([]byte(a.Body))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Tag represents a content tag/category.
type Tag struct {
	ID    int    `json:"id" bson:"id"`
	Label string `json:"label" bson:"label"`
}

// ArticleWriter handles article persistence operations.
type ArticleWriter interface {
	Upsert(ctx context.Context, article *Article) error
	BulkUpsert(ctx context.Context, articles []Article) error
}

// ArticleReader handles article retrieval operations.
type ArticleReader interface {
	GetLastFetched(ctx context.Context, source string) (*Article, error)
}

// HashReader handles content hash retrieval for deduplication.
type HashReader interface {
	GetContentHashes(ctx context.Context, ids []string) (map[string]string, error)
}

// Repository is a composite interface for backward compatibility.
// Services should depend on specific interfaces (ArticleWriter, ArticleReader, HashReader)
// rather than the full Repository when possible.
type Repository interface {
	ArticleWriter
	ArticleReader
	HashReader
}

// Provider defines the interface for external news feed providers.
type Provider interface {
	Crawl(ctx context.Context, handler func([]Article) error) error
	GetName() string
}

// EventProducer publishes article events to a queue.
type EventProducer interface {
	Publish(ctx context.Context, article *Article) error
	PublishBatch(ctx context.Context, articles []Article) error
	Close() error
}

// CMSGateway defines the interface for communicating with the downstream Content Management System.
type CMSGateway interface {
	SyncArticle(ctx context.Context, article *Article) error
}
