#!/usr/bin/env bash
set -euo pipefail
monitoring_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec "$monitoring_dir/../../panl" \
  --endpoint http://127.0.0.1:19090 \
  --grafana http://127.0.0.1:3000 \
  --datasource prometheus \
  --query 'sum by (namespace) (rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[2m]))' \
  "$@"
