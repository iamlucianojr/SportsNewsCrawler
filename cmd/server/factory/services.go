package factory

import (
	"errors"
	"fmt"

	"github.com/SportsNewsCrawler/internal/app"
	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/gateway"
	"github.com/SportsNewsCrawler/internal/infra/queue"
	"github.com/SportsNewsCrawler/internal/infra/repository"
	"github.com/SportsNewsCrawler/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
)

// NewMongoRepository creates a MongoDB repository.
func NewMongoRepository(client *mongo.Client, cfg *config.Config) (domain.Repository, error) {
	if cfg.MongoDBName == "" {
		return nil, errors.New("mongo database name not configured")
	}
	if cfg.MongoColl == "" {
		return nil, errors.New("mongo collection name not configured")
	}
	return repository.NewMongoRepository(client, cfg.MongoDBName, cfg.MongoColl)
}

// NewCMSGateway creates a CMS gateway.
func NewCMSGateway() (domain.CMSGateway, error) {
	return gateway.NewCMSMockGateway(), nil
}

// NewEventProducer wraps the Kafka producer as an EventProducer.
func NewEventProducer(p *queue.KafkaProducer) (domain.EventProducer, error) {
	if p == nil {
		return nil, errors.New("kafka producer is nil")
	}
	return p, nil
}

// NewNewsCrawlerService creates the news crawler service with validation.
func NewNewsCrawlerService(
	repo domain.Repository,
	providers []domain.Provider,
	eventProducer domain.EventProducer,
	cfg *config.Config,
) (*app.NewsCrawlerService, error) {
	if repo == nil {
		return nil, errors.New("repository is nil")
	}
	if len(providers) == 0 {
		return nil, errors.New("no providers configured")
	}
	if eventProducer == nil {
		return nil, errors.New("event producer is nil")
	}
	// The instruction implies that batchSize should be a direct parameter or derived.
	// Assuming the instruction meant to update the validation for cfg.BatchSize.
	if cfg.BatchSize < 1 || cfg.BatchSize > 20000 {
		return nil, fmt.Errorf("invalid batch size: %d (must be 1-20000)", cfg.BatchSize)
	}
	if cfg.WorkerPoolSize <= 0 || cfg.WorkerPoolSize > 100 {
		return nil, fmt.Errorf("invalid worker pool size: %d (must be 1-100)", cfg.WorkerPoolSize)
	}

	return app.NewNewsCrawlerService(
		repo,
		providers,
		eventProducer,
		cfg.PollInterval,
		cfg.BatchSize,
		cfg.WorkerPoolSize,
	), nil
}

// NewCMSSyncService creates the CMS sync service.
func NewCMSSyncService(consumer *queue.KafkaConsumer, gateway domain.CMSGateway) (*app.CMSSyncService, error) {
	if consumer == nil {
		return nil, errors.New("kafka consumer is nil")
	}
	if gateway == nil {
		return nil, errors.New("CMS gateway is nil")
	}
	return app.NewCMSSyncService(consumer, gateway), nil
}
