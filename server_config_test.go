package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentTokenPersistence(t *testing.T) {
	t.Setenv("PANL_AGENT_TOKEN", "")
	path := filepath.Join(t.TempDir(), "token")
	a, e := agentToken(path)
	if e != nil || len(a) != 64 {
		t.Fatal("token generation failed", e)
	}
	b, e := agentToken(path)
	if e != nil || a != b {
		t.Fatal("token changed", e)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0600 {
		t.Fatal(st.Mode())
	}
}
func TestViewerCannotConfigure(t *testing.T) {
	s := testDashboardServer(t)
	for _, path := range []string{"/api/dashboards", "/api/dashboards/kubernetes/undo", "/mcp"} {
		r := httptest.NewRequest("POST", path, nil)
		w := httptest.NewRecorder()
		s.routes(t.TempDir()).ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	s.readOnly = true
	r := httptest.NewRequest("POST", "/api/dashboards", nil)
	r.Header.Set("Authorization", "Bearer test-agent-token-with-enough-length")
	w := httptest.NewRecorder()
	s.routes(t.TempDir()).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestTrailingJSONCannotMutate(t *testing.T) {
	s := testDashboardServer(t)
	d := defaultDashboard()
	d.ID = "trailing"
	b, _ := json.Marshal(d)
	r := httptest.NewRequest("POST", "/api/dashboards", strings.NewReader(string(b)+" {}"))
	r.Header.Set("Authorization", "Bearer test-agent-token-with-enough-length")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.routes(t.TempDir()).ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	if _, e := s.load("trailing"); e == nil {
		t.Fatal("malformed request changed state")
	}
}
