package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func testDashboardServer(t *testing.T) *dashboardServer {
	t.Helper()
	db, e := openDashboardDB(filepath.Join(t.TempDir(), "dashboard.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	s := &dashboardServer{db: db, agentToken: "test-agent-token-with-enough-length", slots: make(chan struct{}, 2)}
	if _, e = s.save(defaultDashboard()); e != nil {
		t.Fatal(e)
	}
	return s
}
func sendJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Authorization", "Bearer test-agent-token-with-enough-length")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestDashboardRevisionAndUndo(t *testing.T) {
	s := testDashboardServer(t)
	h := s.routes(t.TempDir())
	d, e := s.load("kubernetes")
	if e != nil {
		t.Fatal(e)
	}
	original := d.Panels[0].Title
	d.Panels[0].Title = "CPU by pod"
	w := sendJSON(t, h, "PATCH", "/api/dashboards/kubernetes/panels/cpu", map[string]any{"revision": 1, "panel": d.Panels[0]})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = sendJSON(t, h, "PATCH", "/api/dashboards/kubernetes/panels/cpu", map[string]any{"revision": 1, "panel": d.Panels[0]})
	if w.Code != 409 {
		t.Fatal("stale edit accepted", w.Code)
	}
	w = sendJSON(t, h, "POST", "/api/dashboards/kubernetes/undo", map[string]int{"revision": 2})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	got, _ := s.load("kubernetes")
	if got.Revision != 3 || got.Panels[0].Title != original {
		t.Fatal(got)
	}
}
func TestDashboardValidation(t *testing.T) {
	d := defaultDashboard()
	if e := validateDashboard(d); e != nil {
		t.Fatal(e)
	}
	d.Panels[0].Queries[0].Expr = "rate("
	if e := validateDashboard(d); e == nil {
		t.Fatal("invalid PromQL accepted")
	}
	d = defaultDashboard()
	d.Panels[1].ID = d.Panels[0].ID
	if e := validateDashboard(d); e == nil {
		t.Fatal("duplicate ID accepted")
	}
}
func TestDashboardPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")
	db, e := openDashboardDB(path)
	if e != nil {
		t.Fatal(e)
	}
	s := &dashboardServer{db: db}
	d := defaultDashboard()
	d.ID = "test"
	if _, e = s.save(d); e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = openDashboardDB(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s.db = db
	got, e := s.load("test")
	if e != nil || got.Revision != 1 {
		t.Fatal(got, e)
	}
}
func TestDashboardCrossOrigin(t *testing.T) {
	s := testDashboardServer(t)
	r := httptest.NewRequest("POST", "http://localhost/api/dashboards", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://other.example")
	r.Header.Set("Authorization", "Bearer test-agent-token-with-enough-length")
	w := httptest.NewRecorder()
	s.routes(t.TempDir()).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestWebQueryNamespaceAndNulls(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expr := r.URL.Query().Get("query")
		if !strings.Contains(expr, `namespace=~"prod\\.test"`) {
			t.Error(expr)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"namespace":"prod.test"},"values":[[100,"NaN"],[101,"0"],[102,"+Inf"]]}]}}`))
	}))
	defer prom.Close()
	s := testDashboardServer(t)
	s.api = client{endpoint: prom.URL}
	w := sendJSON(t, s.routes(t.TempDir()), "POST", "/api/query", map[string]any{"expr": `up{namespace=~"$namespace"}`, "namespace": "prod.test", "start": 100, "end": 103, "points": 600})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var data struct {
		Series []webSeries `json:"series"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &data); e != nil {
		t.Fatal(e)
	}
	p := data.Series[0].Points
	if p[0].Value != nil || p[1].Value == nil || *p[1].Value != 0 || p[2].Value != nil {
		t.Fatal(p)
	}
}

func TestUndoDoesNotRedo(t *testing.T) {
	s := testDashboardServer(t)
	d, _ := s.load("kubernetes")
	d.Title = "edit one"
	d, e := s.save(d)
	if e != nil {
		t.Fatal(e)
	}
	d.Title = "edit two"
	d, e = s.save(d)
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.undo(d.ID, d.Revision)
	if e != nil || d.Title != "edit one" {
		t.Fatal(d, e)
	}
	d, e = s.undo(d.ID, d.Revision)
	if e != nil || d.Title != "Kubernetes" {
		t.Fatal(d, e)
	}
	if _, e = s.undo(d.ID, d.Revision); e == nil {
		t.Fatal("undo would redo an undone edit")
	}
}

func TestDeleteRejectsStaleRevisionAndRecreateRetainsHistory(t *testing.T) {
	s := testDashboardServer(t)
	if e := s.deleteDashboard("kubernetes", 2); e == nil {
		t.Fatal("stale deletion accepted")
	}
	if e := s.deleteDashboard("kubernetes", 1); e != nil {
		t.Fatal(e)
	}
	if _, e := s.load("kubernetes"); e == nil {
		t.Fatal("dashboard not deleted")
	}
	d, e := s.save(defaultDashboard())
	if e != nil || d.Revision != 2 {
		t.Fatal(d, e)
	}
	if _, e = s.undo(d.ID, d.Revision); e == nil {
		t.Fatal("undo traversed deleted dashboard history")
	}
}
