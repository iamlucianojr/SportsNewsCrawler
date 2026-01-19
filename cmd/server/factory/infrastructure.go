// Package factory provides dependency injection constructors for infrastructure components.
package factory

import (
	"context"
	"errors"
	"time"

	"github.com/SportsNewsCrawler/internal/infra/queue"
	"github.com/SportsNewsCrawler/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/fx"
)

// NewMongoClient creates a MongoDB client with lifecycle management.
func NewMongoClient(lc fx.Lifecycle, cfg *config.Config) (*mongo.Client, error) {
	if cfg.MongoURI == "" {
		return nil, errors.New("mongo URI not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Disconnect(ctx)
		},
	})

	return client, nil
}

// NewMainKafkaProducer creates the main Kafka producer for publishing articles.
func NewMainKafkaProducer(cfg *config.Config, lc fx.Lifecycle) (*queue.KafkaProducer, error) {
	if len(cfg.KafkaBrokers) == 0 {
		return nil, errors.New("kafka brokers not configured")
	}
	if cfg.KafkaTopic == "" {
		return nil, errors.New("kafka topic not configured")
	}

	producer := queue.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return producer.Close()
		},
	})
	return producer, nil
}

// NewDLQProducer creates a Kafka producer for the Dead Letter Queue.
func NewDLQProducer(cfg *config.Config, lc fx.Lifecycle) (*queue.KafkaProducer, error) {
	if len(cfg.KafkaBrokers) == 0 {
		return nil, errors.New("kafka brokers not configured")
	}
	if cfg.KafkaDLQTopic == "" {
		return nil, errors.New("kafka DLQ topic not configured")
	}

	producer := queue.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaDLQTopic)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return producer.Close()
		},
	})
	return producer, nil
}

// NewKafkaConsumer creates a Kafka consumer with DLQ support.
func NewKafkaConsumer(
	cfg *config.Config,
	dlqProducer *queue.KafkaProducer,
	lc fx.Lifecycle,
) (*queue.KafkaConsumer, error) {
	if len(cfg.KafkaBrokers) == 0 {
		return nil, errors.New("kafka brokers not configured")
	}
	if cfg.KafkaTopic == "" {
		return nil, errors.New("kafka topic not configured")
	}

	consumer := queue.NewKafkaConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, "cms-sync-group", dlqProducer)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return consumer.Close()
		},
	})
	return consumer, nil
}
