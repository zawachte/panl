# Local metrics environment

Single-node kind v1.34.0 cluster named `promql`, using rootless Podman. kube-prometheus-stack chart **93.1.0**, release `monitoring`, namespace `monitoring`.

Prometheus: http://127.0.0.1:19090 (9090 is used by Cockpit on this host).
Grafana: http://127.0.0.1:3000, username `admin`; generated password is in the ignored `grafana-admin-password` file beside this README.

Start the TUI with real namespace CPU metrics:

```sh
# From the repository root
./dev/monitoring/explore.sh
```

Press Ctrl+R, then Esc to navigate. Allow a few minutes after installation for rate queries to accumulate samples. Start with `up` if rate queries are still empty. There is no port-forward process to keep alive: kind maps localhost ports to the services' NodePorts.

Queries to try:

```promql
up
sum by (namespace) (rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[2m]))
sum by (namespace) (container_memory_working_set_bytes{container!="",container!="POD"})
rate(prometheus_http_requests_total[2m])
rate(node_network_receive_bytes_total{device!="lo"}[2m])
kube_pod_status_phase{phase="Running"}
```

For A/B, duplicate with `b`, switch to B, edit a different expression, and rerun. Use comparable units: e.g. CPU usage in `monitoring` versus `kube-system`.

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
