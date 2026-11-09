# Security model

The browser is a read-only operational view. Anyone who can reach the viewer can read its dashboards and query its configured Prometheus endpoint. Deploy behind your organization's existing access controls when metrics are private.

Agent configuration is separately protected. All MCP requests and dashboard mutations require a bearer credential of at least 24 characters. Helm generates one in a Kubernetes Secret; use an existing Secret for GitOps. The credential is never included in browser assets, local storage, URLs, dashboard definitions, or logs. Use TLS when sending credentials over a network.

`--read-only` disables configuration access entirely and is intended for public demos. It is not multi-tenant authorization. This release has no per-user roles, viewer login, OAuth server, or per-dashboard ACLs.

The agent can query the configured endpoint and modify dashboard definitions. It cannot run shell commands, access arbitrary endpoints, alter workloads, read Kubernetes Secrets, or execute JavaScript through dashboard definitions. PromQL is parsed before evaluation; the React renderer treats label/title content as text. The Pod does not mount a service-account token and does not require Kubernetes API RBAC.

Queries have response-size limits, a maximum seven-day window, a bounded sample target, a 100-series response cap, deadlines and a six-query concurrency bound. The deployment has memory/CPU limits. These controls do not make an unrestricted public production Prometheus proxy appropriate; use network policy and your upstream's own query controls.

The Helm chart runs as a non-root user with a read-only root filesystem and dropped capabilities. SQLite data is writable only through the persistent data mount. The app is single-replica; use volume snapshots/current-definition exports for backups.

Before reporting a vulnerability, remove credentials and sensitive metric/label values from reproductions. A private reporting address will be supplied with the public repository; until then report privately to the project owner rather than publishing secrets in an issue.
