package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var colors = []string{"81", "213", "156", "221", "111", "209"}

type queryMsg struct {
	id      int
	results []result
	err     error
}
type metaMsg struct {
	values []string
	err    error
}
type model struct {
	completionPrefix string
	completion       bool

	api                             client
	editors                         [2]textarea.Model
	active                          int
	compare, editing, instant, busy bool
	start, end                      time.Time
	width, height, mode, selected   int
	generation                      int
	results                         []result
	hidden                          map[string]bool
	status                          string
	store                           saved
	browser                         string
	items                           []string
	item                            int
	vp                              viewport.Model
	grafana, uid                    string
}

func newModel(c client, grafana, uid, q string) model {
	now := time.Now()
	m := model{api: c, editing: true, start: now.Add(-time.Hour), end: now, width: 110, height: 38, hidden: map[string]bool{}, store: loadState(), grafana: grafana, uid: uid, vp: viewport.New(100, 20)}
	for i := range m.editors {
		e := textarea.New()
		e.SetWidth(100)
		e.SetHeight(4)
		e.CharLimit = 0
		e.ShowLineNumbers = true
		e.Placeholder = "Write a PromQL expression…"
		m.editors[i] = e
	}
	m.editors[0].SetValue(q)
	m.editors[0].Focus()
	return m
}
func (m model) Init() tea.Cmd { return textarea.Blink }
func (m *model) run() tea.Cmd {
	m.generation++
	id := m.generation
	qs := []string{m.editors[0].Value()}
	if m.compare {
		qs = append(qs, m.editors[1].Value())
	}
	for _, q := range qs {
		if strings.TrimSpace(q) == "" {
			m.status = "Query is empty"
			return nil
		}
		m.store.History = append([]string{q}, m.store.History...)
	}
	if len(m.store.History) > 100 {
		m.store.History = m.store.History[:100]
	}
	if e := saveState(m.store); e != nil {
		m.status = e.Error()
	}
	m.busy = true
	c, start, end, instant := m.api, m.start, m.end, m.instant
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		var out []result
		for i, q := range qs {
			r, e := c.query(ctx, q, string(rune('A'+i)), start, end, instant)
			if e != nil {
				return queryMsg{id: id, err: e}
			}
			out = append(out, r)
		}
		return queryMsg{id: id, results: out}
	}
}
func (m model) all() []series {
	var s []series
	for _, r := range m.results {
		s = append(s, r.Series...)
	}
	return s
}
func (m *model) metadata(path string) tea.Cmd {
	m.completion = false
	m.browser = "metadata"
	m.items = nil
	m.item = 0
	m.status = "Loading metadata…"
	c := m.api
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		v, e := c.metadata(ctx, path)
		return metaMsg{v, e}
	}
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		for i := range m.editors {
			m.editors[i].SetWidth(max(20, v.Width-6))
		}
		m.vp.Width = max(10, v.Width-4)
		m.vp.Height = max(3, v.Height-15)
	case queryMsg:
		if v.id != m.generation {
			return m, nil
		}
		m.busy = false
		if v.err != nil {
			m.status = v.err.Error()
		} else {
			m.results = v.results
			m.selected = 0
			m.status = fmt.Sprintf("%d series · %s", len(m.all()), time.Now().Format("15:04:05"))
			for _, r := range v.results {
				if len(r.Warnings) > 0 {
					m.status += " · " + strings.Join(r.Warnings, "; ")
				}
			}
			m.vp.GotoTop()
		}
	case metaMsg:
		if v.err != nil {
			m.status = v.err.Error()
		} else {
			m.items = v.values
			if m.completion && m.completionPrefix != "" {
				filtered := []string{}
				for _, s := range m.items {
					if strings.HasPrefix(s, m.completionPrefix) {
						filtered = append(filtered, s)
					}
				}
				m.items = filtered
			}
			sort.Strings(m.items)
			m.status = fmt.Sprintf("%d entries · ↑↓ select · Enter insert · Esc close", len(m.items))
		}
	case tea.KeyMsg:
		k := v.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if k == "ctrl+r" {
			return m, m.run()
		}
		if m.browser != "" {
			switch k {
			case "esc":
				m.browser = ""
			case "up", "k":
				m.item = max(0, m.item-1)
			case "down", "j":
				m.item = min(len(m.items)-1, m.item+1)
			case "enter":
				if m.browser != "history" && m.browser != "favorites" && m.browser != "metadata" {
					return m, nil
				}
				if m.item >= 0 && m.item < len(m.items) {
					s := m.items[m.item]
					if m.browser == "history" || m.browser == "favorites" {
						m.editors[m.active].SetValue(s)
					} else {
						if m.completion {
							for range []rune(m.completionPrefix) {
								m.editors[m.active], _ = m.editors[m.active].Update(tea.KeyMsg{Type: tea.KeyBackspace})
							}
						}
						m.editors[m.active].InsertString(s)
						m.completion = false
					}
					m.browser = ""
					m.editing = true
					return m, m.editors[m.active].Focus()
				}
			}
			return m, nil
		}
		if k == "esc" {
			m.editing = false
			m.editors[m.active].Blur()
			return m, nil
		}
		if v.Type == tea.KeyCtrlAt || k == "ctrl+space" || (m.editing && v.Type == tea.KeyTab) {
			return m, m.complete()
		}
		if m.editing {
			var cmd tea.Cmd
			m.editors[m.active], cmd = m.editors[m.active].Update(msg)
			return m, cmd
		}
		switch k {
		case "q":
			return m, tea.Quit
		case "e", "enter":
			m.editing = true
			return m, m.editors[m.active].Focus()
		case "tab":
			if m.compare {
				m.active = 1 - m.active
			}
		case "b":
			m.compare = !m.compare
			if m.compare {
				m.editors[1].SetValue(m.editors[0].Value())
				m.active = 1
			} else {
				m.active = 0
			}
			return m, m.run()
		case "r":
			return m, m.run()
		case "i":
			m.instant = !m.instant
			return m, m.run()
		case "v":
			m.mode = (m.mode + 1) % 3
			m.vp.GotoTop()
		case "j", "down":
			m.selected = min(len(m.all())-1, m.selected+1)
		case "k", "up":
			m.selected = max(0, m.selected-1)
		case " ":
			ss := m.all()
			if m.selected >= 0 && m.selected < len(ss) {
				key := labelName(ss[m.selected])
				m.hidden[key] = !m.hidden[key]
			}
		case "s":
			ss := m.all()
			if m.selected >= 0 && m.selected < len(ss) {
				for i, s := range ss {
					m.hidden[labelName(s)] = i != m.selected
				}
			}
		case "a":
			m.hidden = map[string]bool{}
		case "+", "=", "-", "left", "right":
			d := m.end.Sub(m.start)
			switch k {
			case "+", "=":
				m.start = m.start.Add(d / 4)
				m.end = m.end.Add(-d / 4)
			case "-":
				m.start = m.start.Add(-d / 2)
				m.end = m.end.Add(d / 2)
			case "left":
				m.start = m.start.Add(-d / 2)
				m.end = m.end.Add(-d / 2)
			case "right":
				m.start = m.start.Add(d / 2)
				m.end = m.end.Add(d / 2)
			}
			return m, m.run()
		case "z":
			ss := m.all()
			if m.selected >= 0 && m.selected < len(ss) {
				points := ss[m.selected].Points
				if len(points) > 1 {
					peak := points[0]
					for _, p := range points {
						if p.V > peak.V {
							peak = p
						}
					}
					d := m.end.Sub(m.start) / 8
					m.start = peak.T.Add(-d)
					m.end = peak.T.Add(d)
					return m, m.run()
				}
			}
		case "n":
			m.end = time.Now()
			m.start = m.end.Add(-time.Hour)
			return m, m.run()
		case "l", "x":
			ss := m.all()
			if m.selected >= 0 && m.selected < len(ss) {
				q, e := refine(m.editors[m.active].Value(), ss[m.selected].Labels, k == "x")
				if e != nil {
					m.status = e.Error()
				} else {
					m.editors[m.active].SetValue(q)
					return m, m.run()
				}
			}
		case "m":
			return m, m.metadata("label/__name__/values")
		case "L":
			return m, m.metadata("labels")
		case "h":
			m.browser = "history"
			m.items = m.store.History
			m.item = 0
		case "f":
			m.browser = "favorites"
			m.items = m.store.Favorites
			m.item = 0
		case "F":
			q := m.editors[m.active].Value()
			found := false
			for _, f := range m.store.Favorites {
				if f == q {
					found = true
				}
			}
			if !found {
				m.store.Favorites = append(m.store.Favorites, q)
			}
			if e := saveState(m.store); e != nil {
				m.status = e.Error()
			} else {
				m.status = "Saved favorite"
			}
		case "g":
			qs := []string{m.editors[0].Value()}
			if m.compare {
				qs = append(qs, m.editors[1].Value())
			}
			m.browser = "Grafana Explore link (select/copy from terminal)"
			m.items = []string{exploreLink(m.grafana, m.uid, qs, m.start, m.end)}
			m.item = 0
		case "pgdown", "pgup":
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
	default:
		if m.editing {
			var cmd tea.Cmd
			m.editors[m.active], cmd = m.editors[m.active].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}
func (m *model) complete() tea.Cmd {
	e := m.editors[m.active]
	lines := strings.Split(e.Value(), "\n")
	line := e.Line()
	info := e.LineInfo()
	col := info.StartColumn + info.ColumnOffset
	before := strings.Join(lines[:line], "\n")
	if line > 0 {
		before += "\n"
	}
	r := []rune(lines[line])
	before += string(r[:min(col, len(r))])
	start := len(before)
	for start > 0 {
		c := before[start-1]
		if !(c == '_' || c == ':' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.') {
			break
		}
		start--
	}
	prefix := before[start:]
	path := "label/__name__/values"
	open := strings.LastIndex(before, "{")
	close := strings.LastIndex(before, "}")
	if open > close {
		fragment := before[open+1:]
		if comma := strings.LastIndex(fragment, ","); comma >= 0 {
			fragment = fragment[comma+1:]
		}
		if eq := strings.Index(fragment, "="); eq >= 0 {
			label := strings.Trim(strings.TrimSpace(fragment[:eq]), "!~")
			path = "label/" + url.PathEscape(label) + "/values"
		} else {
			path = "labels"
		}
	}
	cmd := m.metadata(path)
	m.completion = true
	m.completionPrefix = prefix
	return cmd
}
func (m model) View() string {
	if m.width < 50 || m.height < 20 {
		return "Enlarge terminal to at least 50×20. Ctrl+C quits.\n"
	}
	source := m.api.endpoint
	if m.api.demo {
		source = "DEMO · synthetic data"
	}
	state := "RANGE"
	if m.instant {
		state = "INSTANT"
	}
	header := accent.Render("  PROMQL / EXPLORE") + "   " + muted.Render(source) + "\n"
	header += fmt.Sprintf("  %s → %s   %s   %s\n", m.start.Format("Jan 02 15:04:05"), m.end.Format("15:04:05"), state, []string{"GRAPH", "TABLE", "RAW"}[m.mode])
	for i := 0; i < 1+boolInt(m.compare); i++ {
		mark := " "
		if i == m.active {
			mark = "▸"
		}
		header += accent.Render(fmt.Sprintf(" %s Query %c", mark, 'A'+i)) + "\n"
		if i == m.active {
			header += m.editors[i].View() + "\n"
		} else {
			header += muted.Render("   "+strings.ReplaceAll(m.editors[i].Value(), "\n", " ")) + "\n"
		}
	}
	h := max(3, m.height-lipgloss.Height(header)-5)
	var body string
	if m.browser != "" {
		body = accent.Render(m.browser) + "\n"
		if len(m.items) == 0 {
			body += "  No entries"
		}
		start := max(0, m.item-h/2)
		for i := start; i < min(len(m.items), start+h-1); i++ {
			s := "  " + m.items[i]
			if i == m.item {
				s = accent.Render("▸ " + m.items[i])
			}
			body += s + "\n"
		}
	} else {
		switch m.mode {
		case 0:
			body = m.graph(max(10, m.width-12), max(3, h-8))
		case 1:
			body = "QUERY / SERIES                                      LAST           MIN           MAX\n"
			for _, s := range m.all() {
				if m.hidden[labelName(s)] {
					continue
				}
				lo, hi, last := stats(s)
				body += fmt.Sprintf("%s\n    last %-12g min %-12g max %-12g\n", labelName(s), last, lo, hi)
			}
		case 2:
			for i, r := range m.results {
				body += fmt.Sprintf("Query %c\n%s\n", 'A'+i, r.Raw)
			}
		}
		if body == "" {
			body = "  Write a query, then Ctrl+R to investigate.\n  Try --demo for a local synthetic incident."
		}
		if m.mode != 0 {
			m.vp.Height = h
			m.vp.SetContent(body)
			body = m.vp.View()
		}
	}
	status := m.status
	if m.busy {
		status = "Querying… " + status
	}
	focus := "NORMAL"
	if m.editing {
		focus = "EDIT · Esc to navigate"
	}
	footer := muted.Render("  "+focus+"  |  "+status) + "\n"
	footer += muted.Render("  Ctrl+R run · e edit · b A/B · Tab query · v view · i instant · m metrics · L labels") + "\n"
	footer += muted.Render("  j/k series · Space hide · s solo · a all · l/x labels · +/- zoom · ←/→ pan · z spike") + "\n"
	footer += muted.Render("  h history · f favorites · F save · g Grafana · Tab complete (edit) · PgUp/Dn scroll · q quit")
	return header + lipgloss.NewStyle().Height(h).MaxHeight(h).MaxWidth(m.width).Render(body) + "\n" + footer
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func stats(s series) (float64, float64, float64) {
	lo, hi, last := math.Inf(1), math.Inf(-1), math.NaN()
	for _, p := range s.Points {
		if math.IsNaN(p.V) || math.IsInf(p.V, 0) {
			continue
		}
		lo = math.Min(lo, p.V)
		hi = math.Max(hi, p.V)
		last = p.V
	}
	return lo, hi, last
}
func (m model) graph(w, h int) string {
	ss := m.all()
	if len(ss) == 0 {
		return "  No series. Ctrl+R runs the query."
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range ss {
		if m.hidden[labelName(s)] {
			continue
		}
		l, r, _ := stats(s)
		lo = math.Min(lo, l)
		hi = math.Max(hi, r)
	}
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return "  No finite samples visible."
	}
	if lo == hi {
		hi = lo + 1
	}
	grid := make([][]int, h)
	for y := range grid {
		grid[y] = make([]int, w)
		for x := range grid[y] {
			grid[y][x] = -1
		}
	}
	for i, s := range ss {
		if m.hidden[labelName(s)] {
			continue
		}
		px, py := -1, -1
		for _, p := range s.Points {
			if math.IsNaN(p.V) || math.IsInf(p.V, 0) {
				px = -1
				continue
			}
			x := int(p.T.Sub(m.start).Seconds() / m.end.Sub(m.start).Seconds() * float64(w-1))
			if m.instant {
				x = w / 2
			}
			y := h - 1 - int((p.V-lo)/(hi-lo)*float64(h-1))
			if x < 0 || x >= w || y < 0 || y >= h {
				continue
			}
			if px >= 0 && x > px {
				for xx := px + 1; xx < x; xx++ {
					yy := py + (y-py)*(xx-px)/(x-px)
					grid[yy][xx] = i
				}
			}
			grid[y][x] = i
			px, py = x, y
		}
	}
	out := muted.Render(fmt.Sprintf("  max %.5g", hi)) + "\n"
	for _, row := range grid {
		out += "  │"
		for _, v := range row {
			if v < 0 {
				out += " "
			} else {
				out += lipgloss.NewStyle().Foreground(lipgloss.Color(colors[v%len(colors)])).Render("•")
			}
		}
		out += "\n"
	}
	out += muted.Render(fmt.Sprintf("  min %.5g   %s %s\n", lo, m.start.Format("15:04:05"), m.end.Format("15:04:05")))
	start := max(0, m.selected-2)
	for i := start; i < min(len(ss), start+5); i++ {
		mark := " "
		if i == m.selected {
			mark = "▸"
		}
		visible := "●"
		if m.hidden[labelName(ss[i])] {
			visible = "○"
		}
		out += lipgloss.NewStyle().Foreground(lipgloss.Color(colors[i%len(colors)])).Render(mark+" "+visible+" "+legendName(ss[i])) + "\n"
	}
	return out
}
func main() {
	serve := flag.Bool("serve", true, "serve the web dashboard (use --serve=false for legacy TUI)")
	readOnly := flag.Bool("read-only", false, "disable all agent configuration access")
	seed := flag.String("seed", "kubernetes", "initial dashboard: kubernetes or none")
	tokenFile := flag.String("agent-token-file", "", "agent token file (default: agent-token beside database); PANL_AGENT_TOKEN overrides it")
	listen := flag.String("listen", "127.0.0.1:8080", "web dashboard address")
	dbpath := flag.String("db", "data/dashboards.db", "SQLite dashboard database")
	dist := flag.String("web-dist", "web/dist", "built frontend directory")
	endpoint := flag.String("endpoint", "http://localhost:9090", "Prometheus-compatible base URL")
	demo := flag.Bool("demo", false, "use deterministic synthetic samples")
	grafana := flag.String("grafana", "http://localhost:3000", "Grafana base URL")
	uid := flag.String("datasource", "prometheus", "Grafana datasource UID")
	q := flag.String("query", "up", "initial PromQL")
	flag.Parse()
	u, e := url.Parse(*endpoint)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		fmt.Fprintln(os.Stderr, "invalid endpoint URL")
		os.Exit(1)
	}
	if *serve {
		if *seed != "none" && *seed != "kubernetes" {
			fmt.Fprintln(os.Stderr, "seed must be kubernetes or none")
			os.Exit(1)
		}
		token := ""
		if !*readOnly {
			path := *tokenFile
			if path == "" {
				path = filepath.Join(filepath.Dir(*dbpath), "agent-token")
			}
			token, e = agentToken(path)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
		}
		if e := serveDashboard(*listen, *dbpath, *dist, client{endpoint: *endpoint, token: os.Getenv("PROMQL_TOKEN"), demo: *demo}, serverOptions{AgentToken: token, ReadOnly: *readOnly, Seed: *seed}); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	m := newModel(client{*endpoint, os.Getenv("PROMQL_TOKEN"), *demo}, *grafana, *uid, *q)
	if _, e = tea.NewProgram(m, tea.WithAltScreen()).Run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
