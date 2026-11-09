package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type dashboardList struct {
	Dashboards []dashboard `json:"dashboards"`
}
type dashboardID struct {
	ID string `json:"id" jsonschema:"Dashboard id, usually kubernetes"`
}
type dashboardInput struct {
	Dashboard dashboard `json:"dashboard"`
}
type updatePanelInput struct {
	DashboardID string `json:"dashboard_id"`
	Revision    int    `json:"revision" jsonschema:"Expected current dashboard revision; fetch again on conflict"`
	Panel       panel  `json:"panel"`
}
type discoverInput struct {
	Label  string `json:"label,omitempty" jsonschema:"Label whose values to list. Omit or use __name__ to discover metric names"`
	Prefix string `json:"prefix,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type discoverOutput struct {
	Values    []string `json:"values"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
}
type queryToolInput struct {
	Expr      string `json:"expr"`
	Namespace string `json:"namespace,omitempty"`
	Start     int64  `json:"start" jsonschema:"Start of time window as Unix seconds"`
	End       int64  `json:"end" jsonschema:"End of time window as Unix seconds; maximum window seven days"`
}
type queryToolOutput struct {
	Data     any      `json:"data"`
	Warnings []string `json:"warnings"`
	Expr     string   `json:"expr"`
}

func (s *dashboardServer) listDashboards() (dashboardList, error) {
	rows, e := s.db.Query("SELECT id FROM dashboards ORDER BY id")
	if e != nil {
		return dashboardList{}, e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return dashboardList{}, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return dashboardList{}, e
	}
	out := dashboardList{Dashboards: []dashboard{}}
	for _, id := range ids {
		d, e := s.load(id)
		if e != nil {
			return out, e
		}
		out.Dashboards = append(out.Dashboards, d)
	}
	return out, nil
}
func newDashboardMCP(s *dashboardServer) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "panl", Version: "0.1.0"}, &mcp.ServerOptions{Instructions: `Panl is a minimal Kubernetes time-series dashboard. Discover metric names and label values, query actual samples, then create or edit dashboards. Every panel needs a stable lowercase-hyphenated id, title, explicit unit, and one or more PromQL queries. Supported units: cores, bytes, bytes/s, percent (0–100), count, seconds, requests/s. Legend templates use {{namespace}}, {{pod}}, or other returned labels. For shared namespace filters use namespace=~"$namespace" in selectors. A query kind of request or limit renders a dashed overlay; other lines are solid. Fetch the current revision before edits. Updates require that revision and return an incremented revision; retry conflicts only after reading current state. Do not fabricate metric names or treat missing samples/limits as zero. The browser automatically picks up saved edits. The default dashboard is kubernetes. There is no built-in chat or agent handoff.`})
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{Name: "list_dashboards", Description: "List dashboards and their complete panel definitions", Annotations: readOnly}, func(ctx context.Context, r *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, dashboardList, error) {
		out, e := s.listDashboards()
		return nil, out, e
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_dashboard", Description: "Read a dashboard with its current revision before editing", Annotations: readOnly}, func(ctx context.Context, r *mcp.CallToolRequest, in dashboardID) (*mcp.CallToolResult, dashboard, error) {
		d, e := s.load(in.ID)
		return nil, d, e
	})
	mcp.AddTool(server, &mcp.Tool{Name: "create_dashboard", Description: "Create a time-series dashboard. Set revision to 0; ids must be unique."}, func(ctx context.Context, r *mcp.CallToolRequest, in dashboardInput) (*mcp.CallToolResult, dashboard, error) {
		if in.Dashboard.Revision != 0 {
			return nil, dashboard{}, errors.New("new dashboard revision must be zero")
		}
		d, e := s.save(in.Dashboard)
		return nil, d, e
	})
	mcp.AddTool(server, &mcp.Tool{Name: "update_dashboard", Description: "Replace a dashboard definition to add, remove, or reorder panels. Supply its current revision."}, func(ctx context.Context, r *mcp.CallToolRequest, in dashboardInput) (*mcp.CallToolResult, dashboard, error) {
		if in.Dashboard.Revision < 1 {
			return nil, dashboard{}, errors.New("fetch the current dashboard revision first")
		}
		d, e := s.save(in.Dashboard)
		return nil, d, e
	})
	mcp.AddTool(server, &mcp.Tool{Name: "update_panel", Description: "Replace one existing panel without altering other panels. Supply the current dashboard revision."}, func(ctx context.Context, r *mcp.CallToolRequest, in updatePanelInput) (*mcp.CallToolResult, dashboard, error) {
		d, e := s.load(in.DashboardID)
		if e != nil {
			return nil, d, e
		}
		if d.Revision != in.Revision {
			return nil, d, errConflict
		}
		for i, p := range d.Panels {
			if p.ID == in.Panel.ID {
				d.Panels[i] = in.Panel
				d, e = s.save(d)
				return nil, d, e
			}
		}
		return nil, d, fmt.Errorf("panel %q not found; use update_dashboard to add a panel", in.Panel.ID)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "discover_metrics", Description: "Discover real metric names or label values, optionally filtered by prefix. Returns at most 500 entries by default.", Annotations: readOnly}, func(ctx context.Context, r *mcp.CallToolRequest, in discoverInput) (*mcp.CallToolResult, discoverOutput, error) {
		label := in.Label
		if label == "" {
			label = "__name__"
		}
		values, e := s.api.metadata(ctx, "label/"+url.PathEscape(label)+"/values")
		if e != nil {
			return nil, discoverOutput{}, e
		}
		out := discoverOutput{Values: []string{}}
		limit := in.Limit
		if limit <= 0 {
			limit = 500
		}
		limit = min(limit, 5000)
		for _, v := range values {
			if strings.HasPrefix(v, in.Prefix) {
				out.Total++
				if len(out.Values) < limit {
					out.Values = append(out.Values, v)
				}
			}
		}
		out.Truncated = out.Total > len(out.Values)
		return nil, out, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "query_prometheus", Description: "Execute range PromQL against real metrics, capped at 100 series. Samples use Unix seconds; missing and non-finite values are null.", Annotations: readOnly}, func(ctx context.Context, r *mcp.CallToolRequest, in queryInput) (*mcp.CallToolResult, queryOutput, error) {
		out, e := s.evaluateQuery(ctx, in)
		return nil, out, e
	})
	addDashboardAgentTools(server, s)

	return server
}
func (s *dashboardServer) mcpHandler() http.Handler {
	server := newDashboardMCP(s)
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}
