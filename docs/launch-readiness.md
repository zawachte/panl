# Release readiness

Status: **locally verified release candidate; publication and independent-user validation remain**.

The project name is Panl. Kubernetes is the primary distribution/deployment path. The human UI is read-only; agents own all dashboard configuration.

## Verified locally

- [x] Helm installation into kind against real Prometheus metrics.
- [x] Non-root container, read-only root filesystem, resource requests/limits, probes, no service-account credentials.
- [x] PVC-backed dashboard recovery after a pod restart.
- [x] Helm upgrade preserves dashboards and the generated agent credential.
- [x] Unauthenticated dashboard mutations rejected; MCP requires bearer auth.
- [x] Read-only public-demo mode disables configuration access entirely, verified in Kubernetes.
- [x] Empty-dashboard MCP workflow: discover, validate, create, split CPU by pod, add requests/limits, add memory.
- [x] Actual PromQL evaluation in the self-contained synthetic demo.
- [x] Query errors, missing metrics, non-finite samples, and sparse time-series gaps handled explicitly.
- [x] Targeted changes, stale revision rejection, undo and deletion/recreation covered by backend tests.
- [x] Browser tests for read-only UI, mobile layout, shared paused URLs, zoom, series controls, background refresh, stale data, and live agent edits.
- [x] Scripted browser recording of MCP-driven dashboard creation and iteration.
- [x] Local synthetic browser load: six panels × 100 series × 300 samples render in approximately 0.6 seconds on the development host.
- [x] Runtime font/assets self-hosted; browser capture shows no external requests.
- [x] Go race tests, `go vet`, frontend typecheck/build and unit tests pass.
- [x] Updated Go/Prometheus dependencies; `govulncheck` reports no known vulnerabilities.
- [x] npm dependency audit reports zero vulnerabilities.
- [x] Container builds from source; Helm chart packaged.
- [x] Clean source archive independently builds and passes the backend/frontend/browser checks; generated frontend assets are byte-identical.
- [x] Fresh Helm install of the release image in an isolated namespace passes authentication, real demo queries, pod restart, and PVC/credential recovery checks.
- [x] CI definitions cover Go/frontend/browser checks and a fresh kind installation plus PVC recovery.

The browser walkthrough is scripted against actual MCP and the real Prometheus query engine. The developer-agent exercise used real cluster metrics. Neither substitutes for independent client/model trials.

## Before a Show HN submission

- [x] Choose the project name: Panl.
- [ ] Confirm repository destination for publication.
- [ ] Confirm the proposed Apache-2.0 licensing choice.
- [ ] Publish source, image, and chart; replace source-build instructions with a tested public-image install command.
- [ ] Run CI on the actual repository (the workflow has been exercised locally in its component parts, not on GitHub's runners).
- [ ] Ask at least two unfamiliar users to deploy on their own clusters and configure their preferred agents without developer guidance.
- [ ] Validate the documented Codex and Claude Code connection recipes in those users' actual clients.
- [ ] Make a reachable read-only demo or publish the recording with an easy local/Kubernetes demo command.
- [ ] Write the launch explanation personally, describing the problem, boundaries, and what was learned. HN's guidelines ask for human-written submissions and prohibit automated posting.

Show HN should link to something people can try. Keep the source and demo available without an email gate. Explain the actual workflow rather than claiming universal agent reliability or production readiness.

References: [Show HN guidelines](https://news.ycombinator.com/showhn.html), [HN guidelines](https://news.ycombinator.com/newsguidelines.html).

## Reproduce checks

```sh
make build
make test
make browser
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
helm lint deploy/helm/panl
docker build -t panl:0.1.0 .
```

For Kubernetes integration, follow `.github/workflows/ci.yml`: create a fresh kind cluster, load the image, install the Helm chart with `demo=true,seed=none`, port-forward it, and run `scripts/kubernetes-smoke.py`. The smoke script verifies authentication, real query results, pod restart, and PVC/credential persistence. It makes a test dashboard and is intended for a disposable test deployment.
