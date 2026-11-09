package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func labelName(s series) string {
	keys := make([]string, 0, len(s.Labels))
	for k := range s.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, s.Labels[k]))
	}
	return s.Query + " {" + strings.Join(parts, ", ") + "}"
}
func refine(q string, selected map[string]string, exclude bool) (string, error) {
	expr, e := parser.NewParser(parser.Options{}).ParseExpr(q)
	if e != nil {
		return "", e
	}
	count := 0
	parser.Inspect(expr, func(n parser.Node, _ []parser.Node) error {
		v, ok := n.(*parser.VectorSelector)
		if !ok {
			return nil
		}
		count++
		for k, value := range selected {
			if k == "__name__" {
				continue
			}
			var kept []*labels.Matcher
			for _, m := range v.LabelMatchers {
				if m.Name != k {
					kept = append(kept, m)
				}
			}
			typ := labels.MatchEqual
			if exclude {
				typ = labels.MatchNotEqual
			}
			m, err := labels.NewMatcher(typ, k, value)
			if err != nil {
				return err
			}
			v.LabelMatchers = append(kept, m)
		}
		sort.Slice(v.LabelMatchers, func(i, j int) bool { return v.LabelMatchers[i].Name < v.LabelMatchers[j].Name })
		return nil
	})
	if count == 0 {
		return "", fmt.Errorf("query has no vector selector")
	}
	return expr.String(), nil
}

type saved struct {
	History   []string
	Favorites []string
}

func statePath() string {
	p, e := os.UserConfigDir()
	if e != nil {
		return ""
	}
	return filepath.Join(p, "promql-tui", "queries.json")
}
func loadState() saved {
	var s saved
	b, _ := os.ReadFile(statePath())
	json.Unmarshal(b, &s)
	return s
}
func saveState(s saved) error {
	p := statePath()
	if p == "" {
		return fmt.Errorf("no config directory")
	}
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), "queries-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), p)
}
func exploreLink(base, uid string, queries []string, start, end time.Time) string {
	targets := []map[string]any{}
	for i, q := range queries {
		targets = append(targets, map[string]any{"refId": string(rune('A' + i)), "expr": q, "range": true, "datasource": map[string]string{"type": "prometheus", "uid": uid}})
	}
	panes := map[string]any{"main": map[string]any{"datasource": uid, "queries": targets, "range": map[string]string{"from": fmt.Sprint(start.UnixMilli()), "to": fmt.Sprint(end.UnixMilli())}}}
	b, _ := json.Marshal(panes)
	return strings.TrimRight(base, "/") + "/explore?" + url.Values{"schemaVersion": {"1"}, "panes": {string(b)}}.Encode()
}

// Keep identifying labels readable on narrow terminals; query text names the metric.
func legendName(s series) string {
	copy := s
	copy.Labels = map[string]string{}
	for k, v := range s.Labels {
		if k != "__name__" {
			copy.Labels[k] = v
		}
	}
	return labelName(copy)
}
