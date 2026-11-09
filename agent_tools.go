package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type validationInput struct {
	Dashboard dashboard `json:"dashboard"`
	Namespace string    `json:"namespace,omitempty"`
}
type queryDiagnostic struct {
	PanelID  string   `json:"panel_id"`
	Query    int      `json:"query"`
	Status   string   `json:"status"`
	Series   int      `json:"series"`
	Message  string   `json:"message,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}
type validationOutput struct {
	Valid   bool              `json:"valid"`
	Queries []queryDiagnostic `json:"queries"`
}
type undoInput struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}
type serverInfo struct {
	Time               int64    `json:"time"`
	Mode               string   `json:"mode"`
	ReadOnly           bool     `json:"read_only"`
	Units              []string `json:"units"`
	MaxPanels          int      `json:"max_panels"`
	MaxQueriesPerPanel int      `json:"max_queries_per_panel"`
}

func addDashboardAgentTools(server *mcp.Server, s *dashboardServer) {
	read := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(server, &mcp.Tool{Name: "get_server_info", Description: "Get current server time (Unix seconds), data mode and dashboard limits before investigating.", Annotations: read}, func(ctx context.Context, r *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, serverInfo, error) {
		mode := "live"
		if s.api.demo {
			mode = "demo"
		}
		return nil, serverInfo{time.Now().Unix(), mode, s.readOnly, []string{"cores", "bytes", "bytes/s", "percent", "count", "seconds", "requests/s"}, 24, 8}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "validate_dashboard", Description: "Dry-run dashboard queries over the last hour without saving. Discover empty results and query errors before applying edits. Missing data is reported explicitly.", Annotations: read}, func(ctx context.Context, r *mcp.CallToolRequest, in validationInput) (*mcp.CallToolResult, validationOutput, error) {
		out := validationOutput{Valid: true, Queries: []queryDiagnostic{}}
		if e := validateDashboard(in.Dashboard); e != nil {
			return nil, out, e
		}
		now := time.Now().Unix()
		for _, p := range in.Dashboard.Panels {
			for i, q := range p.Queries {
				d := queryDiagnostic{PanelID: p.ID, Query: i, Status: "ok"}
				result, e := s.evaluateQuery(ctx, queryInput{Expr: q.Expr, Namespace: in.Namespace, Start: now - 3600, End: now, Points: 100})
				if e != nil {
					d.Status = "error"
					d.Message = e.Error()
					out.Valid = false
				} else {
					d.Series = len(result.Series)
					d.Warnings = result.Warnings
					finite := false
					for _, series := range result.Series {
						for _, point := range series.Points {
							if point.Value != nil {
								finite = true
								break
							}
						}
					}
					if !finite {
						d.Status = "empty"
						d.Message = "No finite samples. Check metric availability, filters, and scrape history."
						out.Valid = false
					}
				}
				out.Queries = append(out.Queries, d)
			}
		}
		return nil, out, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "undo_dashboard", Description: "Undo the last dashboard configuration edit. Supply the current revision to avoid overwriting a newer edit."}, func(ctx context.Context, r *mcp.CallToolRequest, in undoInput) (*mcp.CallToolResult, dashboard, error) {
		d, e := s.undo(in.ID, in.Revision)
		return nil, d, e
	})
	mcp.AddTool(server, &mcp.Tool{Name: "delete_dashboard", Description: "Delete a dashboard after fetching its current revision. Retains revision history; requires explicit user intent to remove the dashboard."}, func(ctx context.Context, r *mcp.CallToolRequest, in undoInput) (*mcp.CallToolResult, deleteOutput, error) {
		e := s.deleteDashboard(in.ID, in.Revision)
		return nil, deleteOutput{in.ID, e == nil}, e
	})

	mcp.AddTool(server, &mcp.Tool{Name: "add_panel", Description: "Append one panel to an existing dashboard. Preserves all other panels; supply current dashboard revision."}, func(ctx context.Context, r *mcp.CallToolRequest, in updatePanelInput) (*mcp.CallToolResult, dashboard, error) {
		d, e := s.load(in.DashboardID)
		if e != nil {
			return nil, d, e
		}
		if d.Revision != in.Revision {
			return nil, d, errConflict
		}
		for _, p := range d.Panels {
			if p.ID == in.Panel.ID {
				return nil, d, fmt.Errorf("panel %q exists; use update_panel", p.ID)
			}
		}
		d.Panels = append(d.Panels, in.Panel)
		d, e = s.save(d)
		return nil, d, e
	})
}

type deleteOutput struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
