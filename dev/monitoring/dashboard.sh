#!/usr/bin/env bash
set -euo pipefail
monitoring_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$monitoring_dir/../.."
exec ./panl --serve --db dev/monitoring/dashboards.db --endpoint http://127.0.0.1:19090 "$@"
