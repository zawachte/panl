# Panl

This repository contains a minimal Kubernetes/PromQL web dashboard and its earlier TUI prototype.

- Human UI: http://127.0.0.1:8080
- Native MCP endpoint: http://127.0.0.1:8080/mcp (Streamable HTTP)
- Prometheus: http://127.0.0.1:19090
- Current dashboard: `kubernetes`
- Kubernetes is the primary deployment path (`deploy/helm/panl`).
- MCP and all dashboard mutations require `PANL_AGENT_TOKEN`; read the deployment Secret instead of putting credentials in source or browser state.

When asked to create or refine a dashboard, prefer the running MCP tools or HTTP API over editing source code. Fetch its current revision, inspect actual metric names/labels, test PromQL, then update the smallest relevant panel. Changes appear automatically in the browser. Preserve unrelated panels and filters. Explicitly report missing metrics; do not fabricate data or turn missing limits into zeros.

Panel units: `cores`, `bytes`, `bytes/s`, `percent` (0–100), `count`, `seconds`, `requests/s`. Shared namespace selectors use `namespace=~"$namespace"`. Legends use label templates such as `{{namespace}}` or `{{pod}}`. Use stable lowercase/hyphenated panel IDs. Queries with `kind: "request"` or `"limit"` render dashed overlays.

See `WEB.md` for the HTTP contract and startup commands. The human UI is read-only. All configuration changes go through agent tools/APIs. Do not add panel editing, query editors, configuration forms, or undo controls to the UI. The project intentionally has no chat, agent-handoff buttons, or agent reasoning UI. Keep the human UI minimal and charts readable.

For source changes: run `go test -race ./...`, `go vet ./...`; in `web/`, run `npm test`, `npm run build`. Browser checks use `npx playwright test` against the self-contained demo and require no cluster; their database is separate from the user's dashboard database. Do not write tests for cosmetic changes alone. Do not commit runtime databases, kubeconfigs, credentials, or built binaries.
