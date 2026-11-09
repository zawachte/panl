package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPrometheusClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/api/v1/query" || r.URL.Query().Get("query") != "up" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected upstream request: %s", r.URL)
		}
		w.Write([]byte(`{"status":"success","warnings":["partial"],"data":{"resultType":"vector"}}`))
	}))
	defer srv.Close()
	var data struct{ ResultType string }
	warnings, err := (client{endpoint: srv.URL + "/prefix/", token: "secret"}).get(context.Background(), "query", url.Values{"query": {"up"}}, &data)
	if err != nil || data.ResultType != "vector" || len(warnings) != 1 || warnings[0] != "partial" {
		t.Fatalf("data=%+v warnings=%v error=%v", data, warnings, err)
	}
}

func TestPrometheusClientErrors(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`{"status":"error","error":"bad query"}`, "bad query"},
		{`upstream unavailable`, "invalid JSON"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(422); w.Write([]byte(tc.body)) }))
			defer srv.Close()
			var data any
			_, err := (client{endpoint: srv.URL}).get(context.Background(), "query", nil, &data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal(err)
			}
		})
	}
}
