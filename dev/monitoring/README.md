# Local metrics environment

Single-node kind v1.34.0 cluster named `promql`, using rootless Podman. kube-prometheus-stack chart **93.1.0**, release `monitoring`, namespace `monitoring`.

Prometheus: http://127.0.0.1:19090 (9090 is used by Cockpit on this host).
Grafana (optional reference): http://127.0.0.1:3000, username `admin`; retrieve its generated password using the command below.

Run Panl against this stack after building it with `make build`:

```sh
./panl --endpoint http://127.0.0.1:19090
```

Open http://127.0.0.1:8080. Allow a few minutes after installation for rate queries to accumulate samples. Agents can discover and query metrics through Panl's MCP endpoint; see [agent setup](../../docs/agents.md).

The kubeconfig is isolated in this directory. To inspect:

```sh
kubectl --kubeconfig dev/monitoring/kubeconfig get pods -n monitoring
```

Recreate (run from repository root, requires kind, Podman, kubectl, Helm, and network access):

```sh
KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name promql \
  --config dev/monitoring/kind.yaml --kubeconfig dev/monitoring/kubeconfig --wait 180s
helm upgrade --install monitoring kube-prometheus-stack \
  --repo https://prometheus-community.github.io/helm-charts --version 93.1.0 \
  --kubeconfig dev/monitoring/kubeconfig \
  --namespace monitoring --create-namespace \
  --values dev/monitoring/values.yaml --wait --timeout 10m
kubectl --kubeconfig dev/monitoring/kubeconfig -n monitoring \
  get secret monitoring-grafana -o jsonpath='{.data.admin-password}' | base64 --decode
```

Prometheus retains 24 hours in ephemeral storage; this is a disposable local environment. Controller-manager, scheduler, proxy, and etcd monitoring are disabled because kind doesn't expose their metrics by default. Node-exporter measures the kind node's view of the host, rather than a separate physical server.

Delete the entire test cluster and its metrics when finished:

```sh
KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name promql
```

Chart documentation: https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack
