# Configure dashboards with your existing agent

The human UI only reads dashboards. Your agent makes configuration changes over authenticated Streamable HTTP MCP or the equivalent JSON HTTP API. There is no embedded model, chat UI, agent-handoff button, or model-provider dependency.

## Connect

First [deploy the app](kubernetes.md), port-forward it, and load its agent credential into `PANL_AGENT_TOKEN`.

### Codex

```sh
codex mcp add panl --url http://localhost:8080/mcp \
  --bearer-token-env-var PANL_AGENT_TOKEN
```

Or add this to your user-level Codex configuration:

```toml
[mcp_servers.panl]
url = "http://localhost:8080/mcp"
bearer_token_env_var = "PANL_AGENT_TOKEN"
```

Start the client from the shell containing the token environment variable. The configuration stores the variable name, not the token. [Official Codex MCP documentation](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).

### Claude Code

Use a project-local `.mcp.json` containing environment expansion rather than a literal credential:

```json
{
  "mcpServers": {
    "panl": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {"Authorization": "Bearer ${PANL_AGENT_TOKEN}"}
    }
  }
}
```

Launch Claude Code with `PANL_AGENT_TOKEN` set and approve the server in the client's MCP settings. [Official Claude Code MCP documentation](https://code.claude.com/docs/en/mcp).

These configurations follow the clients' documented HTTP interfaces. The MCP protocol and workflows are tested here; an independent user trial in each client is still a release checklist item. Cloud-hosted clients cannot reach your laptop's localhost; use a reachable authenticated endpoint or an approved connection appropriate to your environment.

## First conversation

> Discover the available Kubernetes metrics. Create a namespace resource dashboard with CPU usage and memory. Validate the queries against actual samples before saving. Keep the dashboard readable and preserve missing data.

Then:

> Split CPU usage by pod.
>
> Add CPU requests and limits as dashed overlays. Leave missing limits absent.
>
> Add network traffic. Keep the other panels unchanged.

The browser picks up saved edits automatically. Copy its URL to preserve the dashboard and namespace. Paused/zoomed URLs preserve exact time bounds; live URLs preserve the relative time range.

## Tools

| Tool | Purpose |
| --- | --- |
| `get_server_info` | Current time, data mode, units, and limits |
| `discover_metrics` | Metric names or label values, with prefix filtering |
| `query_prometheus` | Execute actual range PromQL with bounded results |
| `validate_dashboard` | Dry-run queries without saving; report errors and empty samples |
| `list_dashboards`, `get_dashboard` | Inspect current definitions and revisions |
| `create_dashboard` | Create a new stable ID with revision 0 |
| `update_panel` | Replace one panel while preserving others |
| `add_panel` | Append one panel |
| `update_dashboard` | Reorder, remove panels, or replace dashboard configuration |
| `undo_dashboard` | Undo the previous configuration edit |
| `delete_dashboard` | Delete a dashboard using its current revision |

Use the server's own time rather than guessing Unix timestamps. Use `validate_dashboard` for concise diagnostics before retrieving large query outputs. An `empty` diagnostic is evidence of missing samples, not proof of zero resource consumption.

Fetch the latest revision before each mutation. A conflict means another actor changed the dashboard: fetch again and reapply only the intended edit. Retrying stale writes blindly is incorrect. Dashboard IDs and panel IDs are stable, lowercase, and hyphenated. Never put credentials in dashboard definitions.

Legends interpolate sample labels such as `{{namespace}}` or `{{pod}}`. Namespace-aware selectors use `namespace=~"$namespace"`. Supported units: cores, bytes, bytes/s, percent (0–100), count, seconds, requests/s. Query `kind: "request"` or `"limit"` produces dashed overlays. When dividing usage by limits, match labels explicitly and let absent limits remain absent.

There is no screenshot tool built into the backend. An agent with browser access can open the dashboard URL and inspect the rendered charts using its normal browser tools. Unit validation and live samples do not replace visual review.

## Smoke test without a model

The included MCP client exercises the real authenticated protocol:

```sh
PANL_URL=http://localhost:8080 python3 scripts/mcp.py list
PANL_URL=http://localhost:8080 python3 scripts/mcp.py call get_server_info <<<'{}'
```

It reads `PANL_AGENT_TOKEN`, or `PANL_TOKEN_FILE` for local development. Tool input comes from stdin, so secrets do not need to be embedded in command arguments. The [agent exercise](agent-workflow.md) records what was verified against real cluster metrics.
