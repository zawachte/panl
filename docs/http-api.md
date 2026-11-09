# HTTP API

All bodies and successful API responses are JSON. Agent configuration requests require `Content-Type: application/json` and `Authorization: Bearer <agent token>`. Browser reads/query requests do not need that credential. Cross-origin configuration writes are rejected.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/dashboards` | All current definitions |
| POST | `/api/dashboards` | Create; revision must be 0 |
| GET | `/api/dashboards/{id}` | Definition and current revision |
| PUT | `/api/dashboards/{id}` | Replace at the current revision |
| PATCH | `/api/dashboards/{id}/panels/{panel-id}` | Replace one panel; body `{revision, panel}` |
| DELETE | `/api/dashboards/{id}` | Delete at current `{revision}` |
| POST | `/api/dashboards/{id}/undo` | Undo previous edit at current `{revision}` |
| GET | `/api/metadata?label=namespace` | Label values; omit label for metric names |
| POST | `/api/query` | Execute `{expr,namespace,start,end,points}` |
| GET | `/api/health` | Upstream connectivity and demo flag |
| GET | `/healthz`, `/readyz` | Process liveness / database readiness |

Unknown fields and trailing JSON values are rejected. Invalid definitions return 400; stale revisions return 409; unauthenticated configuration returns 401; a read-only deployment rejects configuration with 403. Upstream query failures return 502. Query windows use Unix seconds, must be positive, and cannot exceed seven days. The default is 600 target samples per series, bounded to 100–1500; the step is at least one second.

At most six queries execute concurrently. Responses are capped at 100 series, with a warning when truncated. Non-finite samples and missing points are null. Native histogram samples require a PromQL conversion such as `histogram_avg` before graphing.

```json
{
  "id": "namespace-resources",
  "title": "Namespace resources",
  "description": "CPU and memory",
  "revision": 0,
  "panels": [{
    "id": "cpu",
    "title": "CPU usage",
    "unit": "cores",
    "queries": [{
      "expr": "sum by (namespace) (rate(container_cpu_usage_seconds_total{namespace=~\"$namespace\",container!=\"\",image!=\"\"}[5m]))",
      "legend": "{{namespace}}"
    }]
  }]
}
```

Up to 24 panels, eight queries per panel. Empty dashboards are valid. Panel order follows array order. Supported units: `cores`, `bytes`, `bytes/s`, `percent` (0–100), `count`, `seconds`, `requests/s`. Query `kind` can be `request` or `limit` for dashed overlays. Namespace substitution is escaped as an exact regex match; empty namespace selects all. Agents must include the namespace placeholder in queries intended to follow that filter.

The browser never sends a dashboard mutation. It polls for configuration updates every three seconds; chart refresh is every 15 seconds while live. Shareable URLs store dashboard, namespace, relative range, and absolute bounds when paused. Credentials are never part of those URLs.
