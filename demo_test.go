package main

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
)

func TestDemoActuallyEvaluatesPromQL(t *testing.T) {
	c := client{demo: true}
	for _, tc := range []struct {
		expr   string
		series int
	}{
		{`sum by(namespace)(rate(container_cpu_usage_seconds_total[5m]))`, 3},
		{`sum(kube_pod_container_resource_limits{resource="cpu"})`, 1},
		{`up{namespace="does-not-exist"}`, 0},
	} {
		var data struct {
			Result []json.RawMessage `json:"result"`
		}
		_, e := c.get(context.Background(), "query", url.Values{"query": {tc.expr}}, &data)
		if e != nil {
			t.Fatal(e)
		}
		if len(data.Result) != tc.series {
			t.Fatalf("%s: got %d want %d", tc.expr, len(data.Result), tc.series)
		}
	}
	var data any
	if _, e := c.get(context.Background(), "query", url.Values{"query": {"rate("}}, &data); e == nil {
		t.Fatal("invalid PromQL accepted")
	}
}
