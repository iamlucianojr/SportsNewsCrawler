# Grafana Quick Start

## First Time Setup

After starting the services with `make dev` or `docker-compose up -d`:

1. **Wait 30 seconds** for Grafana to fully start
2. Open http://localhost:3000
3. Login with:
   - Username: `admin`
   - Password: `admin`
4. **Dashboard will load automatically!**

## Pre-configured Dashboard

The "Sports News Crawler - Overview" dashboard includes:

### Metrics Panels

1. **Article Ingestion Rate** - Articles processed per minute by provider
2. **Total Articles Ingested** - Cumulative count
3. **Total Articles Published** - Articles sent to Kafka
4. **Provider Fetch Duration** - Response times (p50, p95)
5. **Error Rate** - Errors per minute by provider
6. **HTTP Request Rate** - API endpoint traffic
7. **Service Health** - Up/Down status

### Features

- **Auto-refresh**: Every 5 seconds
- **Time range**: Last 15 minutes
- **Dark theme**: Professional look
- **Legends**: Show last value, mean, max

## Troubleshooting

### "No Data" in Panels

**Check Prometheus is scraping**:
```bash
# Verify Prometheus targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
```

**Check metrics exist**:
```bash
# Query Prometheus directly
curl 'http://localhost:9090/api/v1/query?query=up'
```

**Restart services**:
```bash
docker-compose restart app prometheus grafana
```

### Dashboard Not Loading

```bash
# Check Grafana logs
docker logs sportsnewscrawler-grafana

# Verify dashboard file exists
ls -la config/grafana/dashboards/sports-news-crawler.json

# Restart Grafana
docker-compose restart grafana
```

### Datasource Connection Failed

```bash
# Check Prometheus is accessible from Grafana
docker exec sportsnewscrawler-grafana wget -O- http://prometheus:9090/-/healthy

# Check datasource config
cat config/grafana/datasources/prometheus.yml
```

## Generating Test Data

To see metrics in Grafana, the app needs to be running and processing articles:

```bash
# Check app is running
docker logs sportsnewscrawler-app | tail -20

# Trigger manual fetch (if needed)
curl http://localhost:8080/health

# View metrics endpoint
curl http://localhost:8080/metrics
```

## Expected Metrics

After the app runs for a few minutes, you should see:

- **articles_ingested_total**: Incrementing counter
- **articles_published_total**: Incrementing counter
- **provider_fetch_duration_seconds**: Histogram with buckets
- **http_requests_total**: Counter for /health and /metrics

## Manual Dashboard Creation

If the auto-provisioned dashboard doesn't work:

1. Go to http://localhost:3000
2. Click "+" → "Dashboard"
3. Click "Add new panel"
4. Enter query: `rate(articles_ingested_total[1m])`
5. Click "Apply"

## Useful Queries

Copy these into Grafana panels:

```promql
# Article ingestion rate
rate(articles_ingested_total[1m])

# Total articles
sum(articles_ingested_total)

# Error rate
rate(articles_ingested_total{status=~"error.*"}[1m])

# Provider performance (95th percentile)
histogram_quantile(0.95, rate(provider_fetch_duration_seconds_bucket[5m]))

# Service uptime
up{job="news-crawler"}
```
