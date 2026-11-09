# Kubernetes deployment

Kubernetes is the primary deployment target. One replica serves the read-only viewer, authenticated agent API/MCP, and SQLite-backed dashboard definitions. It queries an existing Prometheus-compatible endpoint; it does not require Kubernetes API credentials.

## Install

The chart and container image are published to GitHub Container Registry:

```sh
helm upgrade --install panl oci://ghcr.io/zawachte/charts/panl --version 0.1.0 \
  --namespace panl --create-namespace \
  --set prometheus.url=http://prometheus-operated.monitoring.svc.cluster.local:9090 \
  --wait
kubectl -n panl port-forward svc/panl 8080:8080
```

Open http://localhost:8080. Change the Prometheus URL to match your cluster. The bundled namespace dashboard expects cAdvisor metrics from kubelet and kube-state-metrics. Agents can build dashboards for any available PromQL metrics.

For an offline kind cluster, pull and load the image:

```sh
docker pull ghcr.io/zawachte/panl:0.1.0
kind load docker-image ghcr.io/zawachte/panl:0.1.0 --name YOUR_CLUSTER
```

For Podman-backed kind, use `podman save --format docker-archive` followed by `KIND_EXPERIMENTAL_PROVIDER=podman kind load image-archive`.

## Agent credential

The chart generates a credential in `panl-agent` and preserves it on Helm upgrades. Read it into the environment of your agent client:

```sh
export PANL_AGENT_TOKEN="$(kubectl -n panl get secret panl-agent -o jsonpath='{.data.token}' | base64 --decode)"
```

Viewer requests need no agent token. MCP requests to `/mcp` and all dashboard mutations require `Authorization: Bearer <token>`. The token never belongs in browser storage, dashboard URLs, or frontend code. [Agent setup](agents.md).

For GitOps, provide a Secret yourself so offline Helm rendering cannot regenerate it:

```sh
kubectl -n panl create secret generic panl-credentials --from-literal=token="$(openssl rand -hex 32)"
helm upgrade --install panl oci://ghcr.io/zawachte/charts/panl --version 0.1.0 -n panl \
  --reuse-values --set agent.existingSecret=panl-credentials
```

To rotate credentials, update that Secret and restart the Deployment. Kubernetes environment variables are read at pod startup.

For Prometheus bearer authentication, create a separate Secret containing `token`, then set `prometheus.existingSecret`. HTTPS uses the container's CA trust store. Custom CA configuration is not included in this release.

## Storage and upgrades

The default chart provisions a 1 GiB `ReadWriteOnce` PVC. It retains current dashboards and revision history. An existing claim can be selected with `persistence.existingClaim`; choose a storage class with `persistence.storageClass`. The PVC is retained when uninstalling the Helm release. Deleting the PVC permanently deletes the stored dashboards.

SQLite makes this a **single-replica deployment**. The chart uses `Recreate` so upgrades do not run two writers against the same volume; expect a short interruption. Horizontal scaling is not supported.

Before upgrading, export dashboard definitions:

```sh
curl -fsS http://localhost:8080/api/dashboards > dashboards-backup.json
```

This exports current definitions, not full revision history. Use a storage-provider volume snapshot for the complete database. Restore definitions through agent tools with `revision: 0` for new dashboard IDs; fetch the current revision before replacing an existing ID.

## Ingress

Ingress is optional and disabled by default:

```yaml
ingress:
  enabled: true
  className: nginx
  host: panl.example.com
  tls:
    - secretName: panl-tls
      hosts: [panl.example.com]
```

The viewer exposes metrics to anyone who can reach it. Use your existing Ingress SSO/access controls for private operational data, and TLS for network access. Agent mutations remain separately protected by their bearer token. This release does not provide viewer login, OAuth, tenants, or per-dashboard permissions.

## Demo without Prometheus

```sh
helm upgrade --install panl-demo oci://ghcr.io/zawachte/charts/panl --version 0.1.0 \
  --namespace panl-demo --create-namespace \
  --set demo=true --set readOnly=true \
  --set persistence.enabled=false --wait
kubectl -n panl-demo port-forward svc/panl-demo 8081:8080
```

This runs the real Prometheus query engine against synthetic Kubernetes series. The UI is explicitly labeled **Demo data**. No external Prometheus or model API key is needed. Configuration access is disabled on this public-demo configuration. To practice agent edits in your own demo, omit `readOnly=true` and use its generated agent credential.

Set `seed=none` for an empty workspace that your agent populates from scratch.

For a quick local look at the same demo image before installing a cluster:

```sh
docker run --rm -p 127.0.0.1:8080:8080 ghcr.io/zawachte/panl:0.1.0 --demo --read-only
```

This local demo is ephemeral; Kubernetes with a PVC remains the primary deployment path.

## Health and operations

- `/healthz`: process liveness.
- `/readyz`: database readiness.
- `/api/health`: upstream query connectivity (or demo engine health).

Prometheus outages do not cause Kubernetes to restart an otherwise healthy viewer. Query calls have deadlines, bounded concurrency, and a 100-series response cap with a warning. Metrics are not stored in SQLite. Requests are forwarded to Prometheus; use its own query limits for additional controls.

Inspect and restart:

```sh
kubectl -n panl get pods,pvc
kubectl -n panl logs deploy/panl
kubectl -n panl rollout restart deploy/panl
```

## Upgrading an existing Observe installation

Panl is the project's new name. To preserve an existing Observe release's immutable Kubernetes selectors, upgrade the same release and namespace with `--set nameOverride=observe`. The chart then keeps the existing PVC, Secret, and resource identities while running the Panl image. Fresh installs use the `panl` chart and labels by default.

The agent environment variable is now `PANL_AGENT_TOKEN`; the chart supplies it from the existing Secret automatically. Update local client configuration to use the new environment-variable name.
