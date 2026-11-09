package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/prometheus/promql/parser"
)

type panelQuery struct {
	Expr   string `json:"expr"`
	Legend string `json:"legend"`
	Kind   string `json:"kind,omitempty"`
}
type panel struct {
	ID      string       `json:"id"`
	Title   string       `json:"title"`
	Unit    string       `json:"unit"`
	Queries []panelQuery `json:"queries"`
}
type dashboard struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Revision    int     `json:"revision"`
	Panels      []panel `json:"panels"`
}
type dashboardServer struct {
	db         *sql.DB
	api        client
	slots      chan struct{}
	agentToken string
	readOnly   bool
}

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func defaultDashboard() dashboard {
	ns := `namespace=~"$namespace"`
	container := ns + `,container!="",container!="POD",image!=""`
	return dashboard{ID: "kubernetes", Title: "Kubernetes", Description: "Namespace resources", Panels: []panel{
		{"cpu", "CPU usage", "cores", []panelQuery{{`sum by (namespace) (rate(container_cpu_usage_seconds_total{` + container + `}[5m]))`, "{{namespace}}", ""}}},
		{"memory", "Memory working set", "bytes", []panelQuery{{`sum by (namespace) (container_memory_working_set_bytes{` + container + `})`, "{{namespace}}", ""}}},
		{"throttling", "CPU throttling", "percent", []panelQuery{{`100 * sum by (namespace) (rate(container_cpu_cfs_throttled_periods_total{` + container + `}[5m])) / sum by (namespace) (rate(container_cpu_cfs_periods_total{` + container + `}[5m]))`, "{{namespace}}", ""}}},
		{"restarts", "Container restarts", "count", []panelQuery{{`sum by (namespace) (increase(kube_pod_container_status_restarts_total{` + ns + `}[5m]))`, "{{namespace}}", ""}}},
		{"network-rx", "Network receive", "bytes/s", []panelQuery{{`sum by (namespace) (rate(container_network_receive_bytes_total{` + ns + `,pod!="",interface!="lo"}[5m]))`, "{{namespace}}", ""}}},
		{"network-tx", "Network transmit", "bytes/s", []panelQuery{{`sum by (namespace) (rate(container_network_transmit_bytes_total{` + ns + `,pod!="",interface!="lo"}[5m]))`, "{{namespace}}", ""}}},
	}}
}
func validateDashboard(d dashboard) error {
	if d.Revision < 0 {
		return errors.New("revision cannot be negative")
	}
	if !validID.MatchString(d.ID) || strings.TrimSpace(d.Title) == "" || len(d.Title) > 100 || len(d.Panels) > 24 {
		return errors.New("dashboard requires a valid id, title, and 0–24 panels")
	}
	seen := map[string]bool{}
	for _, p := range d.Panels {
		if !validID.MatchString(p.ID) || seen[p.ID] || strings.TrimSpace(p.Title) == "" || len(p.Title) > 120 || len(p.Queries) == 0 || len(p.Queries) > 8 {
			return fmt.Errorf("invalid or duplicate panel %q", p.ID)
		}
		seen[p.ID] = true
		switch p.Unit {
		case "cores", "bytes", "bytes/s", "percent", "count", "seconds", "requests/s":
		default:
			return fmt.Errorf("unsupported unit %q", p.Unit)
		}
		for _, q := range p.Queries {
			if len(q.Expr) > 16384 || len(q.Legend) > 256 {
				return errors.New("query or legend is too long")
			}
			if _, e := parser.NewParser(parser.Options{}).ParseExpr(strings.ReplaceAll(q.Expr, "$namespace", ".*")); e != nil {
				return fmt.Errorf("panel %s: %w", p.ID, e)
			}
			if q.Kind != "" && q.Kind != "limit" && q.Kind != "request" {
				return errors.New("query kind must be limit or request")
			}
		}
	}
	return nil
}
func openDashboardDB(path string) (*sql.DB, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`CREATE TABLE IF NOT EXISTS dashboards (id TEXT PRIMARY KEY, revision INTEGER NOT NULL, body TEXT NOT NULL); CREATE TABLE IF NOT EXISTS revisions (id TEXT NOT NULL, revision INTEGER NOT NULL, body TEXT NOT NULL, PRIMARY KEY(id,revision)); CREATE TABLE IF NOT EXISTS undo_links (id TEXT NOT NULL, revision INTEGER NOT NULL, parent INTEGER NOT NULL, PRIMARY KEY(id,revision));`)
	if e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}

var errConflict = errors.New("dashboard changed; fetch its latest revision and retry")

func (s *dashboardServer) save(d dashboard) (dashboard, error) { return s.saveWithParent(d, nil) }
func (s *dashboardServer) saveWithParent(d dashboard, parent *int) (dashboard, error) {
	if e := validateDashboard(d); e != nil {
		return d, e
	}
	tx, e := s.db.Begin()
	if e != nil {
		return d, e
	}
	defer tx.Rollback()
	var current int
	e = tx.QueryRow("SELECT revision FROM dashboards WHERE id=?", d.ID).Scan(&current)
	if e != nil && e != sql.ErrNoRows {
		return d, e
	}
	if current != d.Revision {
		return d, errConflict
	}
	var latest int
	if e = tx.QueryRow("SELECT COALESCE(MAX(revision),0) FROM revisions WHERE id=?", d.ID).Scan(&latest); e != nil {
		return d, e
	}
	d.Revision = latest + 1
	b, e := json.Marshal(d)
	if e != nil {
		return d, e
	}
	_, e = tx.Exec("INSERT INTO dashboards(id,revision,body) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision, body=excluded.body", d.ID, d.Revision, string(b))
	if e != nil {
		return d, e
	}
	_, e = tx.Exec("INSERT INTO revisions(id,revision,body) VALUES(?,?,?)", d.ID, d.Revision, string(b))
	if e != nil {
		return d, e
	}
	undoParent := current
	if parent != nil {
		undoParent = *parent
	}
	if _, e = tx.Exec("INSERT INTO undo_links(id,revision,parent) VALUES(?,?,?)", d.ID, d.Revision, undoParent); e != nil {
		return d, e
	}
	return d, tx.Commit()
}
func (s *dashboardServer) load(id string) (dashboard, error) {
	var d dashboard
	var b string
	e := s.db.QueryRow("SELECT body FROM dashboards WHERE id=?", id).Scan(&b)
	if e != nil {
		return d, e
	}
	e = json.Unmarshal([]byte(b), &d)
	return d, e
}
func jsonResponse(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, code int, e error) {
	jsonResponse(w, code, map[string]string{"error": e.Error()})
}
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		apiError(w, 400, e)
		return false
	}
	if e := requireEOF(dec); e != nil {
		apiError(w, 400, e)
		return false
	}
	return true
}
func (s *dashboardServer) routes(dist string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/dashboards", func(w http.ResponseWriter, r *http.Request) {
		rows, e := s.db.Query("SELECT body FROM dashboards ORDER BY id")
		if e != nil {
			apiError(w, 500, e)
			return
		}
		defer rows.Close()
		ds := []dashboard{}
		for rows.Next() {
			var b string
			if e = rows.Scan(&b); e != nil {
				apiError(w, 500, e)
				return
			}
			var d dashboard
			if e = json.Unmarshal([]byte(b), &d); e != nil {
				apiError(w, 500, e)
				return
			}
			ds = append(ds, d)
		}
		jsonResponse(w, 200, ds)
	})
	mux.HandleFunc("POST /api/dashboards", func(w http.ResponseWriter, r *http.Request) {
		var d dashboard
		if !decodeBody(w, r, &d) {
			return
		}
		if d.Revision != 0 {
			apiError(w, 400, errors.New("new dashboard revision must be zero"))
			return
		}
		s.writeDashboard(w, d)
	})
	mux.HandleFunc("GET /api/dashboards/{id}", func(w http.ResponseWriter, r *http.Request) {
		d, e := s.load(r.PathValue("id"))
		if e != nil {
			apiError(w, 404, e)
			return
		}
		jsonResponse(w, 200, d)
	})
	mux.HandleFunc("PUT /api/dashboards/{id}", func(w http.ResponseWriter, r *http.Request) {
		var d dashboard
		if !decodeBody(w, r, &d) {
			return
		}
		if d.ID != r.PathValue("id") {
			apiError(w, 400, errors.New("id cannot change"))
			return
		}
		s.writeDashboard(w, d)
	})
	mux.HandleFunc("PATCH /api/dashboards/{id}/panels/{panel}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Revision int   `json:"revision"`
			Panel    panel `json:"panel"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		d, e := s.load(r.PathValue("id"))
		if e != nil {
			apiError(w, 404, e)
			return
		}
		if d.Revision != body.Revision {
			apiError(w, 409, errConflict)
			return
		}
		for i, p := range d.Panels {
			if p.ID == r.PathValue("panel") {
				if body.Panel.ID != p.ID {
					apiError(w, 400, errors.New("panel id cannot change"))
					return
				}
				d.Panels[i] = body.Panel
				s.writeDashboard(w, d)
				return
			}
		}
		apiError(w, 404, errors.New("panel not found"))
	})

	mux.HandleFunc("DELETE /api/dashboards/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Revision int `json:"revision"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		if e := s.deleteDashboard(r.PathValue("id"), body.Revision); e != nil {
			apiError(w, 409, e)
			return
		}
		jsonResponse(w, 200, deleteOutput{r.PathValue("id"), true})
	})
	mux.HandleFunc("POST /api/dashboards/{id}/undo", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Revision int `json:"revision"`
		}
		if !decodeBody(w, r, &body) {
			return
		}
		d, e := s.undo(r.PathValue("id"), body.Revision)
		if e != nil {
			apiError(w, 409, e)
			return
		}
		jsonResponse(w, 200, d)
	})
	mux.HandleFunc("POST /api/query", s.queryHTTP)
	mux.HandleFunc("GET /api/metadata", func(w http.ResponseWriter, r *http.Request) {
		path := "label/__name__/values"
		if label := r.URL.Query().Get("label"); label != "" {
			path = "label/" + url.PathEscape(label) + "/values"
		}
		v, e := s.api.metadata(r.Context(), path)
		if e != nil {
			apiError(w, 502, e)
			return
		}
		jsonResponse(w, 200, v)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if e := s.db.PingContext(r.Context()); e != nil {
			apiError(w, 503, e)
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		var data any
		_, e := s.api.get(r.Context(), "query", url.Values{"query": {"up"}}, &data)
		if e != nil {
			apiError(w, 502, e)
			return
		}
		jsonResponse(w, 200, map[string]any{"status": "connected", "demo": s.api.demo})
	})
	mux.Handle("/mcp", s.mcpHandler())
	mux.Handle("/", http.FileServer(http.Dir(dist)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.URL.Path == "/mcp" || (strings.HasPrefix(r.URL.Path, "/api/dashboards") && r.Method != "GET" && r.Method != "HEAD") {
			if !s.protectAgent(w, r) {
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host {
					apiError(w, 403, errors.New("cross-origin writes are disabled"))
					return
				}
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				apiError(w, 415, errors.New("use application/json"))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *dashboardServer) writeDashboard(w http.ResponseWriter, d dashboard) {
	saved, e := s.save(d)
	if e != nil {
		code := 400
		if errors.Is(e, errConflict) {
			code = 409
		}
		apiError(w, code, e)
		return
	}
	jsonResponse(w, 200, saved)
}

type webSample struct {
	Time  float64  `json:"time"`
	Value *float64 `json:"value"`
}
type webSeries struct {
	Labels map[string]string `json:"labels"`
	Points []webSample       `json:"points"`
}

func serveDashboard(addr, dbpath, dist string, c client, opts serverOptions) error {
	if _, e := os.Stat(filepath.Join(dist, "index.html")); e != nil {
		return fmt.Errorf("frontend unavailable at %s: run npm ci and npm run build in web/ first", dist)
	}
	db, e := openDashboardDB(dbpath)
	if e != nil {
		return e
	}
	defer db.Close()
	s := &dashboardServer{db: db, api: c, slots: make(chan struct{}, 6), agentToken: opts.AgentToken, readOnly: opts.ReadOnly}
	if opts.Seed != "none" {
		if _, e = s.load("kubernetes"); errors.Is(e, sql.ErrNoRows) {
			if _, e = s.save(defaultDashboard()); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
	}
	fmt.Printf("Panl dashboard: http://%s\nPrometheus: %s\n", addr, c.endpoint)
	server := &http.Server{Addr: addr, Handler: s.routes(dist), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server.Shutdown(shutdown)
		case <-done:
		}
	}()
	e = server.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}

func (s *dashboardServer) undo(id string, revision int) (dashboard, error) {
	d, e := s.load(id)
	if e != nil {
		return d, e
	}
	if d.Revision != revision {
		return d, errConflict
	}
	parent, e := s.undoParent(id, revision)
	if e != nil {
		return d, e
	}
	if parent < 1 {
		return d, errors.New("no previous edit to undo")
	}
	var body string
	if e = s.db.QueryRow("SELECT body FROM revisions WHERE id=? AND revision=?", id, parent).Scan(&body); e != nil {
		return d, e
	}
	var prev dashboard
	if e = json.Unmarshal([]byte(body), &prev); e != nil {
		return d, e
	}
	nextParent, e := s.undoParent(id, parent)
	if e != nil {
		return d, e
	}
	prev.Revision = d.Revision
	return s.saveWithParent(prev, &nextParent)
}
func (s *dashboardServer) undoParent(id string, revision int) (int, error) {
	var parent int
	e := s.db.QueryRow("SELECT parent FROM undo_links WHERE id=? AND revision=?", id, revision).Scan(&parent)
	if e == sql.ErrNoRows {
		return revision - 1, nil
	}
	return parent, e
}

func (s *dashboardServer) deleteDashboard(id string, revision int) error {
	if revision < 1 {
		return errors.New("fetch the current dashboard revision before deleting")
	}
	result, e := s.db.Exec("DELETE FROM dashboards WHERE id=? AND revision=?", id, revision)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return errConflict
	}
	return nil
}
