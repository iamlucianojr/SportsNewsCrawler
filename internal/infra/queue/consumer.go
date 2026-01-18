package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/SportsNewsCrawler/internal/infra/metrics"
	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader      *kafka.Reader
	dlqProducer domain.EventProducer
}

func NewKafkaConsumer(brokers []string, topic string, groupID string, dlqProducer domain.EventProducer) *KafkaConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	slog.Info("Kafka Consumer initialized", "brokers", brokers, "topic", topic, "group", groupID)
	return &KafkaConsumer{
		reader:      r,
		dlqProducer: dlqProducer,
	}
}

type MessageHandler func(ctx context.Context, article *domain.Article) error

func (c *KafkaConsumer) Start(ctx context.Context, handler MessageHandler) {
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			slog.Error("Error reading kafka message", "error", err)
			break
		}

		var article domain.Article
		if err := json.Unmarshal(m.Value, &article); err != nil {
			slog.Error("Error unmarshaling article", "error", err)
			continue
		}

		slog.Debug("Received article from Kafka", "id", article.ID, "partition", m.Partition)

		if err := handler(ctx, &article); err != nil {
			slog.Error("Error handling article event", "id", article.ID, "error", err)

			// Publish to Dead Letter Queue
			if c.dlqProducer != nil {
				slog.Info("Publishing failed event to DLQ", "article_id", article.ID)
				if dlqErr := c.dlqProducer.Publish(ctx, &article); dlqErr != nil {
					slog.Error("Failed to publish to DLQ", "article_id", article.ID, "error", dlqErr)
				} else {
					metrics.DLQMessagesPublished.WithLabelValues(article.Source).Inc()
				}
			}
		}
	}
}

func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}
