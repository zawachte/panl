# Contributing

The project is an agent-configured dashboard viewer: humans read charts; agents own configuration. Keep that boundary. Changes that add dashboard authoring forms, query editors, chat, or agent-handoff UI do not fit the current scope.

Kubernetes is the primary deployment target. Time-series charts use uPlot; the Go backend forwards live Prometheus queries or evaluates synthetic demo data with the actual Prometheus engine. SQLite stores definitions and revisions, not metrics.

Build/test instructions are in [WEB.md](WEB.md). Run Go tests with the race detector and `go vet`, plus frontend tests/build. Browser tests run against a self-contained demo; no credentials, cluster, or model key are needed. Kubernetes integration uses kind and the Helm chart.

Add tests for meaningful behavior changes, especially query semantics, agent mutation conflicts, persistence, authentication, and chart interactions. Do not add tests that only restate CSS or implementation details. Avoid broad source refactors in feature changes.

Do not commit generated assets, database files, agent tokens, kubeconfigs, Prometheus credentials, browser traces, or build artifacts. Proposed changes must preserve unknown/missing metric values as missing, and report query truncation explicitly.
