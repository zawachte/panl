# Agent workflow verification

Verified against the local kind cluster's real Prometheus metrics on November 8, 2026. The agent used authenticated Streamable HTTP MCP; no dashboard editor or direct database modification was involved.

This was a developer-agent exercise, not an independent-user study or a benchmark of autonomous model reliability. The commands can be reproduced with `scripts/mcp.py`; a portable result is in `examples/namespace-resources.json`.

| Step | MCP operations | Observed result |
| --- | --- | --- |
| Start with an empty workspace | `list_dashboards` | Empty list |
| Discover metrics and scope | `get_server_info`, `discover_metrics` | Four CPU metric names and six namespace label values |
| Create namespace CPU dashboard | `validate_dashboard`, `create_dashboard` | Three namespace series; revision 1 |
| Feedback: split CPU by pod | `get_dashboard`, `validate_dashboard`, `update_panel` | 15 pod series; revision 2 |
| Feedback: add requests and limits | `discover_metrics`, `validate_dashboard`, `update_panel` | 15 usage, eight request, one limit series; revision 3 |
| Feedback: add memory without changing CPU | `validate_dashboard`, `add_panel` | Three memory namespace series; revision 4 |
| Investigate an unavailable metric | `validate_dashboard` | `status: empty`, zero series, `valid: false` |

The unequal counts of usage/request/limit series reflect the actual cluster configuration. The agent did not create zero-valued limits for pods without limits. Dashed request/limit overlays remain separate queries with their own legends.

Additional automated checks cover stale revision conflicts, targeted edits preserving unrelated panels, undo, deletion/recreation preserving monotonically increasing revisions, persistence, and unauthenticated write rejection.

Browser checks confirm that external API edits appear on the open read-only dashboard, survive reload, and leave namespace/time controls usable. They also verify exact paused URL restoration, hidden-series persistence across refresh, no canvas replacement during ordinary refresh, explicit stale-data feedback after upstream failure, and mobile overflow.

The synthetic browser load test rendered six panels × 100 series × 300 samples in approximately 0.6 seconds on the development host. This is a local synthetic result, not a production performance guarantee or a server query benchmark.

Still to validate with people outside this project: installation on another cluster, setup in their preferred agent client, and feedback-driven dashboard creation without developer guidance. The client's own policies, tool approvals, model, and browser capabilities affect that experience.
