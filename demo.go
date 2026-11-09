package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/util/annotations"
)

// Demo data is synthetic, but queries execute with the actual Prometheus engine.
// This keeps discovery, rate(), grouping, filters and agent edits honest.
type demoMetric struct {
	labels labels.Labels
	value  func(float64) float64
}

var demoOnce sync.Once
var demoEngine *promql.Engine
var demoMetrics []demoMetric

func initDemo() {
	demoOnce.Do(func() {
		demoEngine = promql.NewEngine(promql.EngineOpts{MaxSamples: 2_000_000, Timeout: 10 * time.Second, LookbackDelta: 5 * time.Minute, EnableAtModifier: true, EnableNegativeOffset: true, NoStepSubqueryIntervalFn: func(int64) int64 { return 30_000 }})
		for ni, ns := range []string{"checkout", "payments", "monitoring"} {
			for i := 0; i < 3; i++ {
				ni, i, ns := ni, i, ns
				base := .04 + float64(ni)*.025 + float64(i)*.012
				offset := float64(i*70 + ni*130)
				common := map[string]string{"namespace": ns, "pod": fmt.Sprintf("%s-%d", ns, i+1), "container": "app", "image": "demo:v1", "job": "demo", "instance": fmt.Sprintf("demo-%d:8080", ni*3+i), "cluster": "demo"}
				add := func(name string, extra map[string]string, f func(float64) float64) {
					ls := map[string]string{}
					for k, v := range common {
						ls[k] = v
					}
					for k, v := range extra {
						ls[k] = v
					}
					ls["__name__"] = name
					demoMetrics = append(demoMetrics, demoMetric{labels.FromMap(ls), f})
				}
				// Smooth periodic pressure with monotonic counters; no implied real incident.
				add("container_cpu_usage_seconds_total", nil, func(t float64) float64 { return base*t + base*110*math.Sin((t+offset)/180) })
				add("container_memory_working_set_bytes", nil, func(t float64) float64 {
					return (120 + float64(ni*40+i*25) + 20*math.Sin((t+offset)/300)) * 1024 * 1024
				})
				add("container_cpu_cfs_periods_total", nil, func(t float64) float64 { return t * 10 })
				add("container_cpu_cfs_throttled_periods_total", nil, func(t float64) float64 { return t*.4 + 35*math.Sin((t+offset)/120) })
				add("kube_pod_container_status_restarts_total", nil, func(t float64) float64 {
					if ns == "checkout" && i == 0 {
						return math.Floor(t / 1800)
					}
					return 0
				})
				add("container_network_receive_bytes_total", map[string]string{"interface": "eth0"}, func(t float64) float64 { return (18000 + float64(ni*5000+i*3000)) * (t + 80*math.Sin((t+offset)/180)) })
				add("container_network_transmit_bytes_total", map[string]string{"interface": "eth0"}, func(t float64) float64 { return (7000 + float64(ni*2000+i*1000)) * (t + 60*math.Sin((t+offset)/180)) })
				add("kube_pod_container_resource_requests", map[string]string{"resource": "cpu", "unit": "core"}, func(float64) float64 { return .1 })
				add("kube_pod_container_resource_requests", map[string]string{"resource": "memory", "unit": "byte"}, func(float64) float64 { return 256 * 1024 * 1024 })
				// One pod intentionally has no limits, so missing-limit handling is testable.
				if i != 2 {
					add("kube_pod_container_resource_limits", map[string]string{"resource": "cpu", "unit": "core"}, func(float64) float64 { return .5 })
					add("kube_pod_container_resource_limits", map[string]string{"resource": "memory", "unit": "byte"}, func(float64) float64 { return 512 * 1024 * 1024 })
				}
				add("up", nil, func(float64) float64 { return 1 })
				add("http_requests_total", map[string]string{"status": "200", "method": "GET"}, func(t float64) float64 { return 20*t + 600*math.Sin((t+offset)/180) })
			}
		}
		sort.Slice(demoMetrics, func(i, j int) bool { return labels.Compare(demoMetrics[i].labels, demoMetrics[j].labels) < 0 })
	})
}

type demoQueryable struct{}
type demoQuerier struct{ mint, maxt int64 }

func (demoQueryable) Querier(mint, maxt int64) (storage.Querier, error) {
	return &demoQuerier{mint, maxt}, nil
}
func (d *demoQuerier) Close() error { return nil }
func (d *demoQuerier) LabelValues(_ context.Context, name string, _ *storage.LabelHints, _ ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	return demoValues(name), nil, nil
}
func (d *demoQuerier) LabelNames(context.Context, *storage.LabelHints, ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	keys := map[string]bool{}
	for _, m := range demoMetrics {
		m.labels.Range(func(l labels.Label) { keys[l.Name] = true })
	}
	out := []string{}
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil, nil
}
func (d *demoQuerier) Select(ctx context.Context, _ bool, _ *storage.SelectHints, matchers ...*labels.Matcher) storage.SeriesSet {
	out := &demoSet{index: -1}
	for _, m := range demoMetrics {
		match := true
		for _, matcher := range matchers {
			if !matcher.Matches(m.labels.Get(matcher.Name)) {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		chunk := chunkenc.NewXORChunk()
		app, e := chunk.Appender()
		if e != nil {
			out.err = e
			return out
		}
		for ts := (d.mint / 30000) * 30000; ts <= d.maxt; ts += 30000 {
			if ctx.Err() != nil {
				out.err = ctx.Err()
				return out
			}
			app.Append(0, ts, m.value(float64(ts)/1000))
		}
		out.series = append(out.series, &storage.SeriesEntry{Lset: m.labels, SampleIteratorFn: chunk.Iterator})
	}
	return out
}

type demoSet struct {
	series []storage.Series
	index  int
	err    error
}

func (d *demoSet) Next() bool                        { d.index++; return d.index < len(d.series) }
func (d *demoSet) At() storage.Series                { return d.series[d.index] }
func (d *demoSet) Err() error                        { return d.err }
func (d *demoSet) Warnings() annotations.Annotations { return nil }
func demoValues(label string) []string {
	initDemo()
	values := map[string]bool{}
	for _, m := range demoMetrics {
		if v := m.labels.Get(label); v != "" {
			values[v] = true
		}
	}
	out := []string{}
	for v := range values {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func demoGet(ctx context.Context, path string, p url.Values, dst any) ([]string, error) {
	initDemo()
	var data any
	var warnings []string
	switch {
	case path == "labels":
		q := demoQuerier{}
		data, _, _ = q.LabelNames(ctx, nil)
	case strings.HasPrefix(path, "label/") && strings.HasSuffix(path, "/values"):
		label, _ := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(path, "label/"), "/values"))
		data = demoValues(label)
	case path == "query" || path == "query_range":
		var q promql.Query
		var e error
		opts := promql.NewPrometheusQueryOpts(false, 5*time.Minute, nil)
		parse := func(key string) time.Time {
			v, _ := strconv.ParseFloat(p.Get(key), 64)
			return time.UnixMilli(int64(v * 1000))
		}
		if path == "query" {
			ts := time.Now()
			if p.Get("time") != "" {
				ts = parse("time")
			}
			q, e = demoEngine.NewInstantQuery(ctx, demoQueryable{}, opts, p.Get("query"), ts)
		} else {
			step, _ := strconv.ParseFloat(p.Get("step"), 64)
			if step < 1 || parse("end").Sub(parse("start")) > 7*24*time.Hour {
				return nil, fmt.Errorf("invalid demo query window")
			}
			q, e = demoEngine.NewRangeQuery(ctx, demoQueryable{}, opts, p.Get("query"), parse("start"), parse("end"), time.Duration(step*float64(time.Second)))
		}
		if e != nil {
			return nil, e
		}
		defer q.Close()
		result := q.Exec(ctx)
		if result.Err != nil {
			return nil, result.Err
		}
		warnings, _ = result.Warnings.AsStrings(p.Get("query"), 10, 0)
		data = map[string]any{"resultType": result.Value.Type(), "result": result.Value}
	default:
		return nil, fmt.Errorf("unsupported demo API %q", path)
	}
	b, e := json.Marshal(data)
	if e != nil {
		return nil, e
	}
	return warnings, json.Unmarshal(b, dst)
}
