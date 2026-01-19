#!/bin/sh

echo "Waiting for Kibana to be ready..."
until curl -s http://kibana:5601/api/status | grep -q '"state":"green"'; do
  sleep 5
done
echo "Kibana is ready."

# Check if index pattern exists
if curl -s "http://kibana:5601/api/saved_objects/index-pattern/filebeat-pattern" | grep -q '"error"'; then
  echo "Creating index pattern filebeat-*..."
  curl -X POST "http://kibana:5601/api/saved_objects/index-pattern/filebeat-pattern" \
       -H "kbn-xsrf: true" \
       -H "Content-Type: application/json" \
       -d @/config/kibana/index-pattern.json
  echo "Index pattern created."
else
  echo "Index pattern already exists."
fi

# Set as default index pattern
echo "Setting default index pattern..."
curl -X POST "http://kibana:5601/api/kibana/settings/defaultIndex" \
     -H "kbn-xsrf: true" \
     -H "Content-Type: application/json" \
     -d '{"value": "filebeat-pattern"}'
