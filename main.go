package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

func main() {
	readOnly := flag.Bool("read-only", false, "disable all agent configuration access")
	seed := flag.String("seed", "kubernetes", "initial dashboard: kubernetes or none")
	tokenFile := flag.String("agent-token-file", "", "agent token file (default: agent-token beside database); PANL_AGENT_TOKEN overrides it")
	listen := flag.String("listen", "127.0.0.1:8080", "web dashboard address")
	dbpath := flag.String("db", "data/dashboards.db", "SQLite dashboard database")
	dist := flag.String("web-dist", "web/dist", "built frontend directory")
	endpoint := flag.String("endpoint", "http://localhost:9090", "Prometheus-compatible base URL")
	demo := flag.Bool("demo", false, "use deterministic synthetic samples")
	flag.Parse()
	u, e := url.Parse(*endpoint)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		fmt.Fprintln(os.Stderr, "invalid endpoint URL")
		os.Exit(1)
	}
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
}
