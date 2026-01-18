# Sports News Crawler

A production-ready microservice for ingesting, normalizing, and distributing sports news articles from multiple sources using event-driven architecture.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Design Decisions](#design-decisions)
- [Project Structure](#project-structure)
- [Features](#features)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Observability](#observability)
- [Deployment](#deployment)
- [Known Limitations](#known-limitations)
- [Future Improvements](#future-improvements)

## Overview

The Sports News Crawler is a scalable microservice that:
- **Ingests** articles from multiple sports news providers (PulseLive, custom feeds)
- **Normalizes** data into a unified schema
- **Deduplicates** content using SHA256 hashing
- **Publishes** events to Kafka for downstream consumers
- **Syncs** articles to a CMS via event-driven processing
- **Monitors** system health with Prometheus metrics and Jaeger tracing

### Key Capabilities

- ✅ Multi-source ingestion with pluggable transformers
- ✅ Event-driven architecture with Kafka
- ✅ Content-based deduplication
- ✅ Dead Letter Queue (DLQ) for failed messages
- ✅ Distributed tracing with OpenTelemetry/Jaeger
- ✅ Prometheus metrics for observability
- ✅ Graceful shutdown with worker pool management
- ✅ Kubernetes-ready with CI/CD pipeline

## Architecture

### System Context

\`\`\`mermaid
graph TB
    subgraph "External Sources"
        PL[PulseLive API]
        MF[Mock Feed]
    end
    
    subgraph "Sports News Crawler"
        ING[Ingestion Service]
        KAFKA[Kafka]
        SYNC[CMS Sync Service]
    end
    
    subgraph "Storage"
        MONGO[(MongoDB)]
    end
    
    subgraph "Downstream"
        CMS[Content Management System]
        DLQ[Dead Letter Queue]
    end
    
    subgraph "Observability"
        PROM[Prometheus]
        JAEGER[Jaeger]
    end
    
    PL -->|Fetch Articles| ING
    MF -->|Fetch Articles| ING
    ING -->|Store| MONGO
    ING -->|Publish Events| KAFKA
    KAFKA -->|Consume| SYNC
    SYNC -->|Sync| CMS
    SYNC -->|Failed Messages| DLQ
    ING -.->|Metrics| PROM
    SYNC -.->|Metrics| PROM
    ING -.->|Traces| JAEGER
    SYNC -.->|Traces| JAEGER
\`\`\`

### Component Architecture

\`\`\`mermaid
graph LR
    subgraph "Ingestion Flow"
        PROVIDER[Provider Loop] -->|Job| WORKER[Worker Pool]
        WORKER -->|Fetch| API[External API]
        API -->|Articles| HASH[Hash Generator]
        HASH -->|Check| REPO[(Repository)]
        REPO -->|Changed?| FILTER{Content Changed?}
        FILTER -->|Yes| KAFKA[Kafka Producer]
        FILTER -->|No| SKIP[Skip]
        FILTER -->|All| UPSERT[Bulk Upsert]
    end
    
    subgraph "Sync Flow"
        KAFKA -->|Event| CONSUMER[Kafka Consumer]
        CONSUMER -->|Process| CMS_SYNC[CMS Gateway]
        CMS_SYNC -->|Success| DONE[Done]
        CMS_SYNC -->|Failure| DLQ_PROD[DLQ Producer]
    end
\`\`\`

### Data Flow

\`\`\`mermaid
sequenceDiagram
    participant Provider
    participant Worker
    participant MongoDB
    participant Kafka
    participant Consumer
    participant CMS
    
    Provider->>Worker: Schedule Job
    Worker->>Provider: FetchLatest()
    Provider-->>Worker: Raw Articles
    Worker->>Worker: Generate ContentHash
    Worker->>MongoDB: GetContentHashes()
    MongoDB-->>Worker: Existing Hashes
    Worker->>Worker: Filter Changed
    Worker->>MongoDB: BulkUpsert(all)
    Worker->>Kafka: Publish(changed only)
    Kafka->>Consumer: Consume Event
    Consumer->>CMS: SyncArticle()
    alt Success
        CMS-->>Consumer: OK
    else Failure
        Consumer->>Kafka: Publish to DLQ
    end
\`\`\`

## Design Decisions

### 1. Event-Driven Architecture (Kafka)

**Decision**: Use Kafka as the message broker between ingestion and synchronization.

**Rationale**:
- **Decoupling**: Ingestion and CMS sync can scale independently
- **Reliability**: Kafka provides durability and replay capabilities
- **Scalability**: Multiple consumers can process events in parallel

**Trade-offs**:
- Added complexity (Zookeeper/Kafka infrastructure)
- Eventual consistency between MongoDB and CMS

### 2. Content-Based Deduplication

**Decision**: Use SHA256 hash of `Title + Source + Summary + Body` to detect changes.

**Rationale**:
- Prevents re-publishing identical content
- Detects actual content updates (e.g., corrections)
- More reliable than timestamp-based detection

**Trade-offs**:
- Timestamp-only changes won't trigger updates
- Computational overhead of hashing

### 3. Independent Provider Scheduling

**Decision**: Each provider runs on its own ticker/goroutine.

**Rationale**:
- Slow providers don't block fast ones
- Better resource utilization
- Easier to add/remove sources dynamically

**Trade-offs**:
- More goroutines (minimal overhead in Go)
- Slightly more complex shutdown logic

### 4. Worker Pool Pattern

**Decision**: Fixed-size worker pool processes jobs from a buffered channel.

**Rationale**:
- Limits concurrent API calls
- Provides backpressure
- Predictable resource usage

**Trade-offs**:
- Jobs can queue if workers are saturated
- Fixed pool size requires tuning

### 5. Hexagonal Architecture

**Decision**: Clean separation: `domain` → `app` → `infra` → `transport`.

**Rationale**:
- Testability (easy to mock infrastructure)
- Flexibility (swap MongoDB for Postgres)
- Maintainability (clear boundaries)

**Trade-offs**:
- More boilerplate
- Steeper learning curve for new developers

## Project Structure

\`\`\`
SportsNewsCrawler/
├── cmd/
│   ├── server/           # Main application entry point
│   └── mock-feed/        # Mock news feed for testing
├── internal/
│   ├── domain/           # Core business entities & interfaces
│   │   ├── article.go    # Article entity
│   │   └── transformer.go
│   ├── app/              # Application services (business logic)
│   │   ├── service.go    # Ingestion service
│   │   └── sync_service.go # CMS sync service
│   ├── infra/            # Infrastructure implementations
│   │   ├── gateway/      # External system adapters (CMS)
│   │   ├── provider/     # News source clients
│   │   ├── repository/   # Data persistence (MongoDB)
│   │   ├── queue/        # Kafka producer/consumer
│   │   ├── transformer/  # Data normalization
│   │   ├── metrics/      # Prometheus metrics
│   │   └── tracing/      # OpenTelemetry setup
│   └── transport/
│       └── http/         # HTTP server (health, metrics)
├── pkg/
│   └── config/           # Configuration management
├── k8s/                  # Kubernetes manifests
├── config/               # Configuration files
│   ├── sources.json      # News sources configuration
│   ├── prometheus.yml    # Prometheus scrape config
│   └── filebeat.yml      # Log shipping config
├── .github/workflows/    # CI/CD pipelines
├── docker-compose.yml    # Local development stack
└── README.md
\`\`\`

## Features

### Fully Implemented

- ✅ **Multi-Source Ingestion**: PulseLive API + Mock Feed
- ✅ **Pluggable Transformers**: Factory pattern for adding new sources
- ✅ **Content Deduplication**: SHA256-based change detection
- ✅ **Event Publishing**: Kafka with ID-based partitioning
- ✅ **CMS Synchronization**: Event-driven with retry via DLQ
- ✅ **Graceful Shutdown**: Worker pool draining
- ✅ **Observability**:
  - Prometheus metrics (ingestion, errors, latency, duplicates)
  - OpenTelemetry distributed tracing
  - Structured JSON logging (slog)
- ✅ **Security**: Non-root Docker containers
- ✅ **CI/CD**: GitHub Actions (test + build + push)
- ✅ **Kubernetes**: Production-ready manifests

### Partially Implemented

- ⚠️ **Authentication**: Endpoints (`/health`, `/metrics`) are unauthenticated
- ⚠️ **Rate Limiting**: No rate limiting on external API calls

## Getting Started

### Prerequisites

- **Docker** & **Docker Compose**
- **Go 1.23+** (for local development)

### Quick Start

1. **Clone the repository**:
   \`\`\`bash
   git clone https://github.com/lucianojr/sports-news-crawler.git
   cd sports-news-crawler
   \`\`\`

2. **Start the stack**:
   \`\`\`bash
   make dev
   # or
   docker-compose up -d
   \`\`\`

3. **Verify services**:
   \`\`\`bash
   make health
   # or
   curl http://localhost:8080/health
   \`\`\`

4. **View logs**:
   \`\`\`bash
   make logs
   \`\`\`

### Makefile Commands

The project includes a comprehensive Makefile for common operations:

\`\`\`bash
make help              # Show all available commands
make build             # Build the application
make test              # Run tests
make test-coverage     # Run tests with coverage report
make lint              # Run golangci-lint
make check             # Run all checks (fmt, vet, lint, test)

# Development
make dev               # Start development environment
make dev-down          # Stop development environment
make run               # Run application locally
make run-mock          # Run mock feed server

# Docker
make docker-up         # Start all services
make docker-down       # Stop all services
make docker-logs       # View logs
make docker-restart    # Restart services

# Kubernetes
make k8s-deploy        # Deploy to Kubernetes
make k8s-delete        # Delete Kubernetes resources
make k8s-logs          # View application logs
make k8s-status        # Show deployment status

# Monitoring
make metrics           # Open Prometheus
make traces            # Open Jaeger UI
make health            # Check application health

# Database & Kafka
make mongo-shell       # Connect to MongoDB
make kafka-topics      # List Kafka topics
make kafka-consume     # Consume from main topic
make kafka-consume-dlq # Consume from DLQ topic
\`\`\`

### Running Tests

\`\`\`bash
make test              # Run all tests
make test-race         # Run with race detector
make test-coverage     # Generate coverage report
\`\`\`

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| \`SERVER_PORT\` | \`8080\` | HTTP server port |
| \`MONGO_URI\` | \`mongodb://mongodb:27017\` | MongoDB connection string |
| \`MONGO_DB_NAME\` | \`news_crawler\` | Database name |
| \`MONGO_COLLECTION\` | \`articles\` | Collection name |
| \`POLL_INTERVAL\` | \`1m\` | Provider polling interval |
| \`BATCH_SIZE\` | \`20\` | Articles per fetch |
| \`WORKER_POOL_SIZE\` | \`5\` | Concurrent workers |
| \`KAFKA_BROKERS\` | \`kafka:29092\` | Kafka broker addresses |
| \`KAFKA_TOPIC\` | \`news_articles\` | Main topic |
| \`KAFKA_DLQ_TOPIC\` | \`news_articles_dlq\` | Dead letter queue topic |
| \`OTEL_EXPORTER_OTLP_ENDPOINT\` | \`localhost:4317\` | Jaeger endpoint |

### Adding a New Source

1. Create a transformer in \`internal/infra/transformer/\`:
   \`\`\`go
   type MySourceTransformer struct{}
   
   func (t *MySourceTransformer) Transform(data []byte) ([]domain.Article, error) {
       // Parse and normalize
   }
   \`\`\`

2. Register in \`internal/infra/transformer/factory.go\`:
   \`\`\`go
   case "mysource":
       return NewMySourceTransformer(), nil
   \`\`\`

3. Add to \`config/sources.json\`:
   \`\`\`json
   {
       "name": "my-source",
       "url": "https://api.example.com/news",
       "transformer": "mysource"
   }
   \`\`\`

## Observability

### Metrics (Prometheus)

Access at \`http://localhost:9090\`

**Key Metrics**:
- \`articles_ingested_total\`: Total articles fetched (by source, status)
- \`articles_duplicates_skipped_total\`: Skipped due to no content change
- \`worker_active_count\`: Active workers
- \`cms_sync_duration_seconds\`: CMS sync latency
- \`dlq_messages_published_total\`: Failed messages sent to DLQ

### Tracing (Jaeger)

Access at \`http://localhost:16686\`

Traces show:
- Provider fetch latency
- Hash computation time
- Kafka publish operations
- CMS sync duration

### Logs

Structured JSON logs via \`slog\`:
\`\`\`bash
docker-compose logs -f app | jq
\`\`\`

## Deployment

### Kubernetes

See [deployment.md](deployment.md) for detailed instructions.

**Quick Deploy**:
\`\`\`bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/config.yaml
kubectl apply -f k8s/mongodb.yaml
kubectl apply -f k8s/kafka.yaml
kubectl apply -f k8s/jaeger.yaml
kubectl apply -f k8s/app.yaml
\`\`\`

### CI/CD

GitHub Actions workflow (\`.github/workflows/ci.yml\`) automatically:
1. Runs tests on PR
2. Builds Docker image on merge to \`main\`
3. Pushes to \`lucianojr/news_crawler:latest\`

**Required Secrets**:
- \`DOCKER_USERNAME\`
- \`DOCKER_PASSWORD\`

## Known Limitations

1. **Single CMS**: Only supports one CMS endpoint (mock)
2. **No Authentication**: Metrics/health endpoints are public
3. **Fixed Worker Pool**: Pool size is static (not auto-scaling)
4. **Basic Error Handling**: DLQ is simple (no retry backoff)
5. **Local Storage**: MongoDB runs as single replica (not HA)

## Future Improvements

### High Priority
- [ ] Add authentication middleware for HTTP endpoints
- [ ] Implement exponential backoff for DLQ retries
- [ ] Add rate limiting per provider
- [ ] Support multiple CMS destinations

### Medium Priority
- [ ] Auto-scaling worker pool based on queue depth
- [ ] GraphQL API for querying articles
- [ ] Webhook support for real-time source updates
- [ ] Article enrichment (sentiment analysis, tagging)

### Low Priority
- [ ] Admin UI for managing sources
- [ ] A/B testing framework for transformers
- [ ] ML-based duplicate detection

---

## License

MIT

## Contact

For questions or support, please open an issue on GitHub.
