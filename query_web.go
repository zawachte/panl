package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
)

const maxQuerySeries = 100

type queryInput struct {
	Expr      string `json:"expr"`
	Namespace string `json:"namespace,omitempty"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	Points    int    `json:"points,omitempty"`
}
type queryOutput struct {
	Series   []webSeries `json:"series"`
	Warnings []string    `json:"warnings"`
	Expr     string      `json:"expr"`
}
type queryFailure struct {
	code int
	err  error
}

func (e queryFailure) Error() string { return e.err.Error() }
func resolveQuery(in queryInput) (string, error) {
	if in.End <= in.Start || in.End-in.Start > 7*86400 || in.Start < 0 {
		return "", errors.New("query window must be positive Unix seconds and at most seven days")
	}
	if len(in.Expr) > 16384 {
		return "", errors.New("query exceeds 16384 characters")
	}
	ns := ".*"
	if in.Namespace != "" {
		ns = regexp.QuoteMeta(in.Namespace)
	}
	expr := strings.ReplaceAll(in.Expr, `"$namespace"`, strconv.Quote(ns))
	if _, e := parser.NewParser(parser.Options{}).ParseExpr(expr); e != nil {
		return "", e
	}
	return expr, nil
}
func (s *dashboardServer) evaluateQuery(ctx context.Context, in queryInput) (queryOutput, error) {
	expr, e := resolveQuery(in)
	if e != nil {
		return queryOutput{}, queryFailure{400, e}
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return queryOutput{}, ctx.Err()
	}
	var data struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric     map[string]string   `json:"metric"`
			Values     [][]json.RawMessage `json:"values"`
			Histograms json.RawMessage     `json:"histograms"`
		} `json:"result"`
	}
	points := in.Points
	if points == 0 {
		points = 600
	}
	points = max(100, min(1500, points))
	step := int64(math.Ceil(max(1, float64(in.End-in.Start)/float64(points))))
	warnings, e := s.api.get(ctx, "query_range", url.Values{"query": {expr}, "start": {strconv.FormatInt(in.Start, 10)}, "end": {strconv.FormatInt(in.End, 10)}, "step": {strconv.FormatInt(step, 10)}, "limit": {strconv.Itoa(maxQuerySeries + 1)}}, &data)
	if e != nil {
		return queryOutput{}, queryFailure{502, e}
	}
	if data.ResultType != "matrix" {
		return queryOutput{}, queryFailure{502, fmt.Errorf("expected matrix result, got %q", data.ResultType)}
	}
	if len(data.Result) > maxQuerySeries {
		warnings = append(warnings, fmt.Sprintf("Showing the first %d series. Narrow the query or use topk().", maxQuerySeries))
		data.Result = data.Result[:maxQuerySeries]
	}
	out := queryOutput{Expr: expr, Series: []webSeries{}, Warnings: warnings}
	for _, row := range data.Result {
		if len(row.Histograms) > 0 {
			out.Warnings = append(out.Warnings, "Native histogram samples cannot be graphed directly; use histogram_avg, histogram_sum, or histogram_quantile.")
		}
		ws := webSeries{Labels: row.Metric, Points: []webSample{}}
		values := map[int64]*float64{}
		for _, pair := range row.Values {
			if len(pair) != 2 {
				continue
			}
			var ts float64
			var text string
			if json.Unmarshal(pair[0], &ts) != nil || json.Unmarshal(pair[1], &text) != nil {
				continue
			}
			var value *float64
			v, e := strconv.ParseFloat(text, 64)
			if e == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
				value = &v
			}
			values[int64(ts)] = value
		}
		for ts := in.Start; ts <= in.End; ts += step {
			ws.Points = append(ws.Points, webSample{float64(ts), values[ts]})
		}
		out.Series = append(out.Series, ws)
	}
	return out, nil
}
func (s *dashboardServer) queryHTTP(w http.ResponseWriter, r *http.Request) {
	var in queryInput
	if !decodeBody(w, r, &in) {
		return
	}
	out, e := s.evaluateQuery(r.Context(), in)
	if e != nil {
		code := 502
		var failure queryFailure
		if errors.As(e, &failure) {
			code = failure.code
		}
		apiError(w, code, e)
		return
	}
	jsonResponse(w, 200, out)
}
