package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type PaginationConfig struct {
	Type         string `json:"type"`        // "page" or "offset"
	PageParam    string `json:"page_param"`  // e.g. "page", "p", "start"
	LimitParam   string `json:"limit_param"` // e.g. "pageSize", "limit"
	DefaultLimit int    `json:"default_limit"`
}

type SourceConfig struct {
	Name        string           `json:"name"`
	URL         string           `json:"url"`
	Transformer string           `json:"transformer"`
	Pagination  PaginationConfig `json:"pagination"`
}

type Config struct {
	MongoURI        string
	MongoDBName     string
	MongoColl       string
	PollInterval    time.Duration
	BatchSize       int
	ServerPort      string
	Sources         []SourceConfig
	WorkerPoolSize  int
	KafkaBrokers    []string
	KafkaTopic      string
	KafkaDLQTopic   string
	SourcesFilePath string
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Parse comma-separated list of brokers
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "kafka:29092"
	}

	cfg := &Config{
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		MongoDBName:     getEnv("MONGO_DB_NAME", "news_crawler"),
		MongoColl:       getEnv("MONGO_COLLECTION", "articles"),
		MongoURI:        getEnv("MONGO_URI", "mongodb://mongodb:27017"),
		PollInterval:    getDurationEnv("POLL_INTERVAL", 1*time.Minute),
		BatchSize:       getIntEnv("BATCH_SIZE", 20),
		WorkerPoolSize:  getIntEnv("WORKER_POOL_SIZE", 5),
		KafkaBrokers:    strings.Split(brokers, ","),
		KafkaTopic:      getEnv("KAFKA_TOPIC", "news_articles"),
		KafkaDLQTopic:   getEnv("KAFKA_DLQ_TOPIC", "news_articles_dlq"),
		SourcesFilePath: getEnv("SOURCES_FILE_PATH", "config/sources.json"),
	}
	cfg.Sources = loadSources(cfg.SourcesFilePath)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadSources(path string) []SourceConfig {
	// If path doesn't exist, try fallback for convenience during dev/test if default was used
	if _, err := os.Stat(path); os.IsNotExist(err) && path == "config/sources.json" {
		fallback := "../config/sources.json"
		if _, err := os.Stat(fallback); err == nil {
			path = fallback
		}
	}

	file, err := os.Open(path)
	if err != nil {
		slog.Warn("Could not open sources.json, using default PulseLive source", "path", path, "error", err)
		return []SourceConfig{
			{
				Name:        "default-pulselive",
				URL:         getEnv("PULSE_API_URL", "https://content-ecb.pulselive.com/content/ecb/text/EN/"),
				Transformer: "pulselive",
			},
		}
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("Failed to close config file", "error", err)
		}
	}()

	var sources []SourceConfig
	if err := json.NewDecoder(file).Decode(&sources); err != nil {
		slog.Error("Error decoding sources.json", "error", err)
		return nil
	}

	validSources := make([]SourceConfig, 0, len(sources))
	for _, s := range sources {
		if err := s.Validate(); err != nil {
			slog.Error("Invalid source config, skipping", "name", s.Name, "error", err)
			continue
		}
		validSources = append(validSources, s)
	}

	if len(validSources) == 0 {
		slog.Warn("No valid sources found in config file")
	}

	return validSources
}

func (s *SourceConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if s.URL == "" {
		return fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(s.URL, "http") {
		return fmt.Errorf("url must start with http/https")
	}
	if s.Transformer == "" {
		return fmt.Errorf("transformer is required")
	}
	return nil
}

func (c *Config) Validate() error {
	if c.MongoURI == "" {
		return fmt.Errorf("MONGO_URI is required")
	}
	if len(c.KafkaBrokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		// Try parsing as duration string (e.g. "1m", "60s")
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		// Try parsing as integer seconds
		if i, err := strconv.Atoi(value); err == nil {
			return time.Duration(i) * time.Second
		}
	}
	return fallback
}
