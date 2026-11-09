# Publishing releases

The [Publish workflow](../.github/workflows/publish.yml) runs for `v*` tags. It validates that the tag, chart version, and chart appVersion agree; runs backend/frontend checks; builds Linux AMD64 and ARM64 images; and pushes the image followed by the OCI Helm chart.

For a release, update `deploy/helm/panl/Chart.yaml` (`version` and `appVersion`), the default image tag in `values.yaml`, and versioned installation examples. Commit and pass CI, then push a matching tag such as `v0.1.1`. Use a new version for each release; do not move published tags. The workflow authenticates with `GITHUB_TOKEN` and `packages: write`; no registry secret is needed.

Packages:

- Container: `ghcr.io/zawachte/panl:VERSION`
- Helm chart: `oci://ghcr.io/zawachte/charts/panl --version VERSION`

The packages are public. Verify anonymous pulls with an empty Helm registry configuration and an unauthenticated OCI client, then install into a disposable namespace using the published chart. `scripts/kubernetes-smoke.py` checks queries, rejected unauthenticated writes, and PVC/credential recovery; it requires `demo=true,seed=none` and a fresh PVC. Never point it at a user's deployment.

## 0.1.0

[Publishing run](https://github.com/zawachte/panl/actions/runs/42554526334) passed. Anonymous pulls and a fresh kind installation from GHCR passed, including the Kubernetes smoke test. AMD64 was executed; ARM64 was built and is present in the image manifest, but was not executed on ARM hardware.

- Image index digest: `sha256:00f93e62fb3a04ac8e44d505fa295566a13a4b29210f0636ff4ef4cb7d8d51d1`
- Chart digest: `sha256:571da122206894b55ed134e8728851b04b3ddf0f8256d3b5ee8687960fb6ccd0`
