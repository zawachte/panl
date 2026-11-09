package main

import (
	"context"
	"encoding/json"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/prometheus/prometheus/promql/parser"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAPI(t *testing.T) {
	for _, instant := range []bool{false, true} {
		t.Run(map[bool]string{false: "range", true: "instant"}[instant], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer secret" {
					t.Error("missing token")
				}
				if r.URL.Query().Get("query") != "up" {
					t.Error("query missing")
				}
				if instant {
					if r.URL.Path != "/prefix/api/v1/query" {
						t.Error(r.URL.Path)
					}
					w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"job":"api"},"value":[100,"1"]}]}}`))
				} else {
					if r.URL.Query().Get("step") == "" {
						t.Error("missing step")
					}
					w.Write([]byte(`{"status":"success","warnings":["partial"],"data":{"resultType":"matrix","result":[{"metric":{"job":"api"},"values":[[100,"1"],[101,"2"]]}]}}`))
				}
			}))
			defer srv.Close()
			r, e := (client{endpoint: srv.URL + "/prefix", token: "secret"}).query(context.Background(), "up", "A", time.Unix(0, 0), time.Unix(120, 0), instant)
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Series) != 1 || r.Series[0].Points[0].V != 1 {
				t.Fatalf("%+v", r)
			}
		})
	}
}
func TestAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"status":"error","error":"bad query"}`))
	}))
	defer srv.Close()
	_, e := (client{endpoint: srv.URL}).query(context.Background(), "bad", "A", time.Now(), time.Now(), true)
	if e == nil || !strings.Contains(e.Error(), "bad query") {
		t.Fatal(e)
	}
}
func TestRefineNested(t *testing.T) {
	for _, exclude := range []bool{false, true} {
		q, e := refine(`sum(rate(http_requests_total{job="old"}[5m]))`, map[string]string{"__name__": "http_requests_total", "job": "new", "instance": "api\"1"}, exclude)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = parser.NewParser(parser.Options{}).ParseExpr(q); e != nil {
			t.Fatal(e)
		}
		if strings.Contains(q, "old") || !strings.Contains(q, "instance") {
			t.Fatal(q)
		}
	}
}
func TestExplore(t *testing.T) {
	link := exploreLink("https://grafana.example/sub", "uid", []string{"up", "rate(x[5m])"}, time.Unix(1, 0), time.Unix(2, 0))
	u, e := url.Parse(link)
	if e != nil {
		t.Fatal(e)
	}
	var panes map[string]any
	if e = json.Unmarshal([]byte(u.Query().Get("panes")), &panes); e != nil {
		t.Fatal(e)
	}
	if len(panes["main"].(map[string]any)["queries"].([]any)) != 2 {
		t.Fatal(link)
	}
}
func TestZoomAndStaleResults(t *testing.T) {
	m := newModel(client{demo: true}, "http://grafana", "uid", "up")
	m.editing = false
	old := m.end.Sub(m.start)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m = next.(model)
	if m.end.Sub(m.start) != old/2 || cmd == nil {
		t.Fatal("zoom did not rerun")
	}
	n, _ := m.Update(queryMsg{id: m.generation - 1, err: context.Canceled})
	if !n.(model).busy {
		t.Fatal("stale result changed state")
	}
}
func TestCompletion(t *testing.T) {
	m := newModel(client{demo: true}, "", "", "http_req")
	cmd := m.complete()
	n, _ := m.Update(cmd())
	m = n.(model)
	if len(m.items) != 1 || m.items[0] != "http_requests_total" {
		t.Fatal(m.items)
	}
	n, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if q := n.(model).editors[0].Value(); q != "http_requests_total" {
		t.Fatal(q)
	}
}
func TestState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := saved{History: []string{"up"}, Favorites: []string{"rate(x[5m])"}}
	if e := saveState(s); e != nil {
		t.Fatal(e)
	}
	got := loadState()
	if len(got.History) != 1 || got.Favorites[0] != s.Favorites[0] {
		t.Fatal(got)
	}
}

func TestCompletionKeyboardShortcuts(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyTab, tea.KeyCtrlAt} {
		t.Run(tea.KeyMsg{Type: key}.String(), func(t *testing.T) {
			m := newModel(client{demo: true}, "", "", "http_req")
			next, cmd := m.Update(tea.KeyMsg{Type: key})
			m = next.(model)
			if cmd == nil || m.browser != "metadata" || !m.completion {
				t.Fatal("shortcut did not open completion")
			}
			next, _ = m.Update(cmd())
			m = next.(model)
			if len(m.items) != 1 || m.items[0] != "http_requests_total" {
				t.Fatal(m.items)
			}
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if q := next.(model).editors[0].Value(); q != "http_requests_total" {
				t.Fatal(q)
			}
		})
	}
}
func TestTabSwitchesQueriesInNormalMode(t *testing.T) {
	m := newModel(client{demo: true}, "", "", "up")
	m.editing = false
	m.compare = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(model)
	if got.active != 1 || got.browser != "" || cmd != nil {
		t.Fatal("normal-mode Tab did not switch queries")
	}
}
