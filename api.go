package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type sample struct {
	T time.Time
	V float64
}
type series struct {
	Labels map[string]string
	Points []sample
	Query  string
}
type result struct {
	Series   []series
	Raw      string
	Warnings []string
}
type client struct {
	endpoint, token string
	demo            bool
}

func (c client) get(ctx context.Context, path string, params url.Values, dst any) ([]string, error) {
	if c.demo {
		return demoGet(ctx, path, params, dst)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.endpoint, "/")+"/api/v1/"+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var env struct {
		Status   string
		Data     json.RawMessage
		Error    string
		Warnings []string
	}
	if err = json.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("HTTP %d: invalid JSON", resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 || env.Status != "success" {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, env.Error)
	}
	return env.Warnings, json.Unmarshal(env.Data, dst)
}
func (c client) query(ctx context.Context, q, ref string, start, end time.Time, instant bool) (result, error) {
	if c.demo {
		var ss []series
		for i := 0; i < 4; i++ {
			s := series{Labels: map[string]string{"__name__": "demo_requests_per_second", "instance": fmt.Sprintf("api-%02d:9090", i+1), "job": "checkout"}, Query: ref}
			for j := 0; j <= 120; j++ {
				v := 20 + float64(i)*9 + 8*math.Sin(float64(j)/9+float64(i))
				if j > 70 && j < 80 {
					v += 40
				}
				if ref == "B" {
					v *= 0.7
				}
				s.Points = append(s.Points, sample{start.Add(end.Sub(start) * time.Duration(j) / 120), v})
			}
			if instant {
				s.Points = s.Points[len(s.Points)-1:]
			}
			ss = append(ss, s)
		}
		b, _ := json.MarshalIndent(ss, "", "  ")
		return result{Series: ss, Raw: string(b)}, nil
	}
	p := url.Values{"query": {q}}
	path := "query_range"
	step := math.Max(1, end.Sub(start).Seconds()/240)
	if instant {
		path = "query"
		p.Set("time", strconv.FormatInt(end.Unix(), 10))
	} else {
		p.Set("start", strconv.FormatInt(start.Unix(), 10))
		p.Set("end", strconv.FormatInt(end.Unix(), 10))
		p.Set("step", strconv.FormatFloat(step, 'f', 3, 64))
	}
	var data struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	}
	warnings, err := c.get(ctx, path, p, &data)
	if err != nil {
		return result{}, err
	}
	raw, _ := json.MarshalIndent(data, "", "  ")
	out := result{Raw: string(raw), Warnings: warnings}
	parse := func(pair []json.RawMessage) (sample, error) {
		if len(pair) != 2 {
			return sample{}, fmt.Errorf("invalid sample")
		}
		var t float64
		var v string
		if e := json.Unmarshal(pair[0], &t); e != nil {
			return sample{}, e
		}
		if e := json.Unmarshal(pair[1], &v); e != nil {
			return sample{}, e
		}
		n, e := strconv.ParseFloat(v, 64)
		return sample{time.UnixMilli(int64(t * 1000)), n}, e
	}
	switch data.ResultType {
	case "matrix", "vector":
		var rows []struct {
			Metric     map[string]string   `json:"metric"`
			Values     [][]json.RawMessage `json:"values"`
			Value      []json.RawMessage   `json:"value"`
			Histogram  json.RawMessage     `json:"histogram"`
			Histograms json.RawMessage     `json:"histograms"`
		}
		if err = json.Unmarshal(data.Result, &rows); err != nil {
			return out, err
		}
		for _, r := range rows {
			s := series{Labels: r.Metric, Query: ref}
			if len(r.Histogram) > 0 || len(r.Histograms) > 0 {
				out.Warnings = append(out.Warnings, "Native histograms are available in raw mode only")
			}
			if len(r.Value) > 0 {
				r.Values = append(r.Values, r.Value)
			}
			for _, p := range r.Values {
				v, e := parse(p)
				if e != nil {
					return out, e
				}
				s.Points = append(s.Points, v)
			}
			out.Series = append(out.Series, s)
		}
	case "scalar":
		var pair []json.RawMessage
		json.Unmarshal(data.Result, &pair)
		v, e := parse(pair)
		if e != nil {
			return out, e
		}
		out.Series = []series{{Labels: map[string]string{}, Points: []sample{v}, Query: ref}}
	case "string":
		out.Warnings = append(out.Warnings, "String result: see raw mode")
	default:
		return out, fmt.Errorf("unsupported result type %q", data.ResultType)
	}
	return out, nil
}
func (c client) metadata(ctx context.Context, path string) ([]string, error) {
	var values []string
	_, e := c.get(ctx, path, nil, &values)
	return values, e
}
