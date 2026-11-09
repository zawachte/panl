# Panl web dashboard

The project is a read-only Kubernetes/PromQL dashboard viewer configured by agents.

- [Kubernetes installation, credentials, storage, and Ingress](docs/kubernetes.md)
- [Agent setup and MCP tools](docs/agents.md)
- [HTTP API and dashboard schema](docs/http-api.md)
- [Verified agent workflow](docs/agent-workflow.md)

Local development:

```sh
cd web
npm ci
npm test
npm run build
cd ..
go test -race ./...
go vet ./...
go build -o panl .
./panl --demo
```

Requires Go 1.27.3+, a C compiler, and Node.js 24 LTS to build. The demo needs no Prometheus. It runs real PromQL against explicitly synthetic metrics. By default, a local server listens on 127.0.0.1:8080 and stores dashboards and a generated agent credential under `data/`. Use `PANL_AGENT_TOKEN` to provide a credential of at least 24 characters instead. `--read-only` disables agent configuration entirely; `--seed=none` starts with no default dashboard.

For real metrics, use `--endpoint=http://localhost:19090`. Existing development data is in `dev/monitoring/dashboards.db`; specify `--db` explicitly to use it. `PROMQL_TOKEN` configures upstream bearer auth.

Run `npm run dev` in `web/` for hot reload while the Go server is running. Vite proxies `/api` to localhost:8080. `npx playwright test` starts a separate demo-backed server and database; it needs no cluster or model key. Go tests and browser tests exercise real parsing/query execution and actual chart interactions.

The original terminal prototype remains accessible with `--serve=false`; it is not the release product or the recommended deployment path.
