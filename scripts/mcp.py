#!/usr/bin/env python3
"""Small authenticated Streamable HTTP MCP client; accepts tool input on stdin."""
import argparse
import json
import os
from pathlib import Path
import sys
import urllib.error
import urllib.request

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("command", choices=["list", "call"])
parser.add_argument("tool", nargs="?")
parser.add_argument("--url", default=os.environ.get("PANL_URL", "http://127.0.0.1:8080"))
args = parser.parse_args()
token = os.environ.get("PANL_AGENT_TOKEN", "").strip()
if not token:
    try:
        token = Path(os.environ.get("PANL_TOKEN_FILE", "data/agent-token")).read_text().strip()
    except FileNotFoundError:
        raise SystemExit("Set PANL_AGENT_TOKEN from the deployment Secret, or PANL_TOKEN_FILE for a local token file.")

def rpc(method, params, request_id):
    req = urllib.request.Request(args.url.rstrip("/") + "/mcp", data=json.dumps({"jsonrpc":"2.0","id":request_id,"method":method,"params":params}).encode(), headers={"Content-Type":"application/json","Accept":"application/json, text/event-stream","Authorization":"Bearer " + token,"MCP-Protocol-Version":"2025-06-18"})
    try:
        with urllib.request.urlopen(req, timeout=120) as response:
            data = json.load(response)
    except urllib.error.HTTPError as e:
        raise SystemExit(f"HTTP {e.code}: {e.read().decode()}") from e
    if "error" in data:
        raise SystemExit(json.dumps(data["error"]))
    return data.get("result", {})

rpc("initialize", {"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"panl-cli","version":"0.1.0"}}, 1)
if args.command == "list":
    output = rpc("tools/list", {}, 2)
else:
    if not args.tool:
        parser.error("call requires a tool name")
    output = rpc("tools/call", {"name":args.tool,"arguments":json.load(sys.stdin)}, 2)
    if output.get("isError"):
        print(json.dumps(output, indent=2))
        raise SystemExit(1)
    output = output.get("structuredContent", output)
print(json.dumps(output, indent=2))
