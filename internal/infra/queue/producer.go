package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/SportsNewsCrawler/internal/domain"
	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{}, // Hash balancer ensures messages with same key go to same partition
	}
	slog.Info("Kafka Producer initialized", "brokers", brokers, "topic", topic)
	return &KafkaProducer{writer: w}
}

func (p *KafkaProducer) Publish(ctx context.Context, article *domain.Article) error {
	payload, err := json.Marshal(article)
	if err != nil {
		return err
	}

	// Use Updated Partitioning Strategy: Key by Article ID to distribute load
	// (Previously Key was source, which caused hotspots)
	msg := kafka.Message{
		Key:   []byte(article.ID),
		Value: payload,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		slog.Error("Failed to write to kafka", "error", err)
		return err
	}

	slog.Debug("Published article to Kafka", "id", article.ID, "source", article.Source)
	return nil
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
