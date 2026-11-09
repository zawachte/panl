package main

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestMCPAgentDashboardWorkflow(t *testing.T) {
	s := testDashboardServer(t)
	server := newDashboardMCP(s)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "1"}, nil)
	a, b := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ss, e := server.Connect(ctx, a, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	cs, e := client.Connect(ctx, b, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	tools, e := cs.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(tools.Tools) != 12 {
		t.Fatal(len(tools.Tools))
	}
	d := defaultDashboard()
	d.ID = "agent-created"
	d.Title = "Agent dashboard"
	res, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "create_dashboard", Arguments: map[string]any{"dashboard": d}})
	if e != nil || res.IsError {
		t.Fatal(res, e)
	}
	d, e = s.load(d.ID)
	if e != nil || d.Revision != 1 {
		t.Fatal(d, e)
	}
	p := d.Panels[0]
	p.Title = "CPU by namespace"
	res, e = cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_panel", Arguments: map[string]any{"dashboard_id": d.ID, "revision": 1, "panel": p}})
	if e != nil || res.IsError {
		t.Fatal(res, e)
	}
	got, _ := s.load(d.ID)
	if got.Revision != 2 || got.Panels[0].Title != p.Title || got.Panels[1].Title != d.Panels[1].Title {
		t.Fatal(got)
	}
	res, e = cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_panel", Arguments: map[string]any{"dashboard_id": d.ID, "revision": 1, "panel": p}})
	if e != nil || !res.IsError {
		t.Fatal("stale agent update accepted", res, e)
	}
}
