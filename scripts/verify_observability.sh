#!/bin/bash

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo "======================================="
echo "Verifying Observability Stack"
echo "======================================="

check_service() {
    name=$1
    url=$2
    expected=$3
    
    echo -n "Checking $name... "
    if curl -s "$url" | grep -q "$expected"; then
        echo -e "${GREEN}OK${NC}"
    else
        echo -e "${RED}FAILED${NC}"
        echo "  Target: $url"
        echo "  Expected to contain: $expected"
    fi
}

check_status() {
    name=$1
    url=$2
    
    echo -n "Checking $name status... "
    status=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    if [ "$status" -eq 200 ]; then
        echo -e "${GREEN}OK (200)${NC}"
    else
        echo -e "${RED}FAILED ($status)${NC}"
        echo "  Target: $url"
    fi
}

# 1. Prometheus
check_status "Prometheus UI" "http://localhost:9090/graph"
check_service "Prometheus Targets" "http://localhost:9090/api/v1/targets" "active"

# 2. Grafana
check_status "Grafana UI" "http://localhost:3000/login"
check_service "Grafana Health" "http://localhost:3000/api/health" "ok"

# 3. Jaeger
check_status "Jaeger UI" "http://localhost:16686/search"

# 4. Elasticsearch
check_service "Elasticsearch" "http://localhost:9200" "You Know, for Search"

# 5. Kibana
check_service "Kibana Status" "http://localhost:5601/api/status" "green"

# 6. Index Pattern (Check if automation worked)
echo -n "Checking Index Patterns... "
if curl -s "http://localhost:5601/api/saved_objects/_find?type=index-pattern" | grep -q "filebeat-*"; then
     echo -e "${GREEN}OK (Found 'filebeat-*')${NC}"
else
     echo -e "${RED}FAILED (Pattern missing)${NC}"
fi

echo "======================================="
echo "Verification Complete"
echo "======================================="
