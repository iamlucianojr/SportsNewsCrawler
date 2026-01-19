package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type ReadinessWaiter struct {
	mongoClient *mongo.Client
	brokers     []string
	topic       string
}

func NewReadinessWaiter(mongoClient *mongo.Client, brokers []string, topic string) *ReadinessWaiter {
	return &ReadinessWaiter{
		mongoClient: mongoClient,
		brokers:     brokers,
		topic:       topic,
	}
}

func (w *ReadinessWaiter) WaitForDependencies(ctx context.Context) error {
	if err := w.waitForMongo(ctx); err != nil {
		return err
	}
	if err := w.waitForKafka(ctx); err != nil {
		return err
	}
	return nil
}

func (w *ReadinessWaiter) waitForMongo(ctx context.Context) error {
	slog.Info("Waiting for MongoDB...")
	// We use a ticker to poll for readiness every 2 seconds.
	// We do not use a timeout here because the service should wait indefinitely (or until context cancel)
	// rather than crashing if dependencies are slow to start (e.g. in dev environment).
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.mongoClient.Ping(ctx, readpref.Primary()); err != nil {
				slog.Warn("MongoDB not ready yet", "error", err)
				continue
			}
			slog.Info("MongoDB is ready")
			return nil
		}
	}
}

func (w *ReadinessWaiter) waitForKafka(ctx context.Context) error {
	slog.Info("Waiting for Kafka...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.checkKafka(ctx); err != nil {
				slog.Warn("Kafka not ready yet", "error", err)
				continue
			}
			slog.Info("Kafka is ready")
			return nil
		}
	}
}

func (w *ReadinessWaiter) checkKafka(ctx context.Context) error {
	// 1. Check TCP connection to brokers
	for _, broker := range w.brokers {
		conn, err := net.DialTimeout("tcp", broker, 2*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to broker %s: %w", broker, err)
		}
		_ = conn.Close()
	}

	// 2. Check if topic exists
	// We verify against the first broker for simplicity
	if len(w.brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}

	conn, err := kafka.Dial("tcp", w.brokers[0])
	if err != nil {
		return fmt.Errorf("failed to dial kafka: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	partitions, err := conn.ReadPartitions(w.topic)
	if err != nil {
		// If topic doesn't exist, this usually returns an error or empty partitions
		// Note: segmentio/kafka-go might return UnknownTopicOrPartition error
		return fmt.Errorf("failed to read partitions for topic %s: %w", w.topic, err)
	}

	if len(partitions) == 0 {
		return fmt.Errorf("topic %s has no partitions", w.topic)
	}

	return nil
}
