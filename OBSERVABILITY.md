# Observability Stack Guide

## Overview

The Sports News Crawler includes a comprehensive observability stack for monitoring, logging, and tracing.

## Services

### 🔍 Grafana (Metrics Visualization)
- **URL**: http://localhost:3000
- **Credentials**: admin / admin
- **Purpose**: Professional metrics dashboards
- **Data Source**: Prometheus

### 📊 Prometheus (Metrics Storage)
- **URL**: http://localhost:9090
- **Purpose**: Time-series metrics database
- **Retention**: 15 days
- **Scrape Interval**: 5s (app), 15s (others)

### 📝 Kibana (Log Visualization)
- **URL**: http://localhost:5601
- **Purpose**: Log search and visualization
- **Data Source**: Elasticsearch
- **Index Pattern**: `sportsnewscrawler-*`

### 🔎 Elasticsearch (Log Storage)
- **URL**: http://localhost:9200
- **Purpose**: Log storage and search
- **Index**: `sportsnewscrawler-YYYY.MM.DD`

### 🔗 Jaeger (Distributed Tracing)
- **URL**: http://localhost:16686
- **Purpose**: Request tracing and performance analysis
- **Protocol**: OTLP (OpenTelemetry)

## Quick Start

```bash
# Start all services
make dev

# Or manually
docker-compose up -d

# Check service status
docker-compose ps
```

## Accessing Dashboards

### Grafana (Recommended for Metrics)
1. Open http://localhost:3000
2. Login with `admin` / `admin`
3. Navigate to Dashboards
4. Explore pre-configured dashboards (when available)

**Key Metrics**:
- `articles_ingested_total` - Total articles processed
- `articles_published_total` - Articles sent to Kafka
- `provider_fetch_duration_seconds` - Provider response times
- `http_requests_total` - HTTP endpoint metrics

### Kibana (Logs)
1. Open http://localhost:5601
2. Go to "Discover"
3. Select index pattern: `sportsnewscrawler-*`
4. Filter logs by:
   - `level`: ERROR, WARN, INFO, DEBUG
   - `provider`: Provider name
   - `service`: news-crawler, mock-feed

**Useful Searches**:
```
level:"ERROR"
provider:"pulselive"
message:"Failed to fetch"
```

### Jaeger (Traces)
1. Open http://localhost:16686
2. Select service: `news-crawler`
3. Click "Find Traces"
4. Explore request flows

**Trace Operations**:
- `processProvider` - Provider fetch and processing
- `HTTP GET /health` - Health check requests
- `HTTP GET /metrics` - Metrics endpoint

## Prometheus Queries

### Common Queries

**Article Ingestion Rate**:
```promql
rate(articles_ingested_total[5m])
```

**Error Rate**:
```promql
rate(articles_ingested_total{status="error_fetch"}[5m])
```

**Provider Performance**:
```promql
histogram_quantile(0.95, rate(provider_fetch_duration_seconds_bucket[5m]))
```

**Active Providers**:
```promql
count(up{job="news-crawler"})
```

## Log Structure

Application logs are structured JSON:

```json
{
  "time": "2026-01-18T22:00:00Z",
  "level": "INFO",
  "msg": "Fetched page",
  "provider": "pulselive",
  "page": 0,
  "articles_on_page": 20,
  "total_articles": 20
}
```

**Key Fields**:
- `level`: Log level (DEBUG, INFO, WARN, ERROR)
- `provider`: News provider name
- `page`: Pagination page number
- `error`: Error message (if any)

## Troubleshooting

### Grafana Not Showing Data
```bash
# Check Prometheus is running
curl http://localhost:9090/-/healthy

# Check Grafana datasource
docker logs sportsnewscrawler-grafana
```

### Kibana Index Pattern Missing
```bash
# Check Elasticsearch indices
curl http://localhost:9200/_cat/indices

# Check Filebeat is running
docker logs sportsnewscrawler-filebeat
```

### No Traces in Jaeger
```bash
# Check Jaeger is receiving traces
curl http://localhost:16686/api/services

# Check app OTLP configuration
docker logs sportsnewscrawler-app | grep -i trace
```

## Data Retention

| Service | Retention | Storage |
|---------|-----------|---------|
| Prometheus | 15 days | 10GB max |
| Elasticsearch | Indefinite* | Volume-based |
| Jaeger | In-memory | Lost on restart |

*Configure index lifecycle management for production

## Performance Impact

| Service | CPU | Memory | Disk I/O |
|---------|-----|--------|----------|
| Grafana | Low | ~100MB | Low |
| Prometheus | Low | ~200MB | Medium |
| Elasticsearch | Medium | ~512MB | High |
| Kibana | Low | ~200MB | Low |
| Jaeger | Low | ~100MB | Low |
| Filebeat | Very Low | ~50MB | Low |

## Production Recommendations

1. **Prometheus**:
   - Increase retention to 30-90 days
   - Add Alertmanager for alerts
   - Configure remote write for long-term storage

2. **Elasticsearch**:
   - Increase heap size for production load
   - Configure index lifecycle management
   - Set up index rollover

3. **Jaeger**:
   - Use persistent storage (Elasticsearch/Cassandra)
   - Configure sampling rate
   - Set up trace retention policies

4. **Grafana**:
   - Configure SMTP for alerts
   - Set up user authentication (LDAP/OAuth)
   - Create custom dashboards for your metrics

## Useful Commands

```bash
# View all logs
make logs

# View specific service logs
docker logs -f sportsnewscrawler-app

# Restart observability stack
docker-compose restart prometheus grafana kibana

# Clear all data (WARNING: destructive)
docker-compose down -v
```

## Dashboard Examples

### Grafana Dashboard Panels

**Article Processing Rate**:
- Query: `rate(articles_ingested_total[5m])`
- Type: Graph
- Unit: ops/sec

**Provider Health**:
- Query: `up{job="news-crawler"}`
- Type: Stat
- Thresholds: 0 (red), 1 (green)

**Error Rate**:
- Query: `rate(articles_ingested_total{status=~"error.*"}[5m])`
- Type: Graph
- Alert: > 0.1 errors/sec

### Kibana Visualizations

**Error Logs Over Time**:
- Type: Line chart
- Y-axis: Count
- X-axis: @timestamp
- Filter: level:"ERROR"

**Top Error Messages**:
- Type: Data table
- Metrics: Count
- Buckets: message.keyword
- Filter: level:"ERROR"

## Next Steps

1. Create custom Grafana dashboards for your metrics
2. Set up Kibana saved searches for common queries
3. Configure alerting rules in Prometheus
4. Add custom log fields for better filtering
