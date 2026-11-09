import { StrictMode, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from "@tanstack/react-query";
import {
  Activity,
  ArrowLeft,
  ChevronDown,
  Pause,
  Play,
  RefreshCw,
} from "lucide-react";
import { api } from "./data";
import type { Dashboard, Window } from "./data";
import { PanelCard } from "./PanelCard";
import { readView, viewSearch } from "./view";
import "@fontsource-variable/dm-sans/wght.css";
import "@fontsource/ibm-plex-mono/latin-400.css";
import "./style.css";
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 10_000 },
  },
});
function App() {
  const explicitDashboard = useRef(
    new URLSearchParams(globalThis.location.search).has("dashboard"),
  ).current;
  const initial = useRef(readView(globalThis.location.search)).current;
  const [dashboardId, setDashboardId] = useState(initial.dashboardId);
  const [namespace, setNamespace] = useState(initial.namespace);
  const [duration, setDuration] = useState(initial.duration);
  const [live, setLive] = useState(initial.live);
  const [window, setWindow] = useState<Window>(initial.window);
  const [back, setBack] = useState<Window[]>([]);
  const expandedRef = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const dashboards = useQuery({
    queryKey: ["dashboards"],
    queryFn: ({ signal }) =>
      api<Dashboard[]>("/api/dashboards", undefined, "GET", signal),
    refetchInterval: 3000,
  });
  const dashboard = dashboards.data?.find((d) => d.id === dashboardId);
  const namespaces = useQuery({
    queryKey: ["namespaces"],
    queryFn: ({ signal }) =>
      api<string[]>("/api/metadata?label=namespace", undefined, "GET", signal),
    refetchInterval: 60000,
  });
  const health = useQuery({
    queryKey: ["health"],
    queryFn: ({ signal }) =>
      api<{ status: string; demo: boolean }>(
        "/api/health",
        undefined,
        "GET",
        signal,
      ),
    refetchInterval: 30000,
  });
  useEffect(() => {
    history.replaceState(
      null,
      "",
      viewSearch({ dashboardId, namespace, duration, live, window }),
    );
  }, [dashboardId, namespace, duration, live, window]);
  useEffect(() => {
    const pop = () => {
      const v = readView(globalThis.location.search);
      setDashboardId(v.dashboardId);
      setNamespace(v.namespace);
      setDuration(v.duration);
      setLive(v.live);
      setWindow(v.window);
      setBack([]);
    };
    globalThis.addEventListener("popstate", pop);
    return () => globalThis.removeEventListener("popstate", pop);
  }, []);
  useEffect(() => {
    if (
      dashboards.data?.length &&
      !dashboards.data.some((d) => d.id === dashboardId) &&
      !explicitDashboard
    ) {
      setDashboardId(dashboards.data[0].id);
    }
  }, [dashboards.data, dashboardId, explicitDashboard]);
  function refresh() {
    queryClient.invalidateQueries({ queryKey: ["panel-metrics"] });
    const end = Math.floor(Date.now() / 1000);
    setWindow({ start: end - duration, end });
    setBack([]);
  }
  useEffect(() => {
    if (!live) return;
    const timer = setInterval(() => {
      const end = Math.floor(Date.now() / 1000);
      setWindow({ start: end - duration, end });
    }, 15000);
    return () => clearInterval(timer);
  }, [live, duration]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") setExpanded(null);
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, []);
  useEffect(() => {
    if (!expanded) return;
    const previous = document.activeElement as HTMLElement | null;
    expandedRef.current?.querySelector<HTMLButtonElement>("button")?.focus();
    return () => {
      if (previous?.isConnected) previous.focus();
    };
  }, [expanded]);
  function zoom(next: Window) {
    setBack((old) => [...old, window]);
    setWindow(next);
    setLive(false);
  }
  const expandedPanel = dashboard?.panels.find((p) => p.id === expanded);
  const timeLabel = (t: number) =>
    new Date(t * 1000).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
  return (
    <div className="app">
      <header className="topbar">
        <div className="identity">
          <a className="brand" href="/" aria-label="Panl home">
            <span className="brand-mark">
              <Activity size={19} />
            </span>
            panl
          </a>
          <span className="brand-separator">/</span>
          {(dashboards.data?.length || 0) > 1 ||
          (!!dashboards.data?.length && !dashboard) ? (
            <select
              className="dashboard-select"
              aria-label="Dashboard"
              value={dashboardId}
              onChange={(e) => {
                setDashboardId(e.target.value);
                setExpanded(null);
              }}
            >
              {dashboards.data?.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.title}
                </option>
              ))}
            </select>
          ) : (
            <h1>
              {dashboard?.title ||
                (dashboards.data?.length === 0 ? "Dashboards" : "Dashboard")}
            </h1>
          )}
          <span
            className="connection"
            role="status"
            title={
              health.isError
                ? "Prometheus unavailable"
                : health.isPending
                  ? "Connecting to Prometheus"
                  : "Prometheus connected"
            }
          >
            <span
              className={`status-dot ${health.isError ? "disconnected" : health.isPending ? "pending" : ""}`}
            />
            <span className="sr-only">
              {health.isError
                ? "Prometheus unavailable"
                : health.isPending
                  ? "Connecting to Prometheus"
                  : "Prometheus connected"}
            </span>
          </span>
          {health.data?.demo && (
            <span
              className="demo-label"
              title="Synthetic Kubernetes metrics evaluated with the Prometheus query engine"
            >
              Demo data
            </span>
          )}
        </div>
        <div className="header-controls">
          <div className="filter">
            <label className="sr-only" htmlFor="namespace">
              Namespace
            </label>
            <div className="select-wrap">
              <select
                id="namespace"
                value={namespace}
                onChange={(e) => setNamespace(e.target.value)}
              >
                <option value="">All namespaces</option>
                {namespaces.data?.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
              <ChevronDown size={13} />
            </div>
          </div>
          <div className="time-controls">
            {back.length > 0 && (
              <>
                <button
                  className="icon-button back-button"
                  title="Previous time range"
                  aria-label="Previous time range"
                  onClick={() => {
                    setWindow(back[back.length - 1]);
                    setBack(back.slice(0, -1));
                  }}
                >
                  <ArrowLeft size={15} />
                </button>
              </>
            )}
            <div className="select-wrap">
              <select
                aria-label="Time range"
                value={live ? duration : "custom"}
                onChange={(e) => {
                  const d = Number(e.target.value);
                  setDuration(d);
                  const end = Math.floor(Date.now() / 1000);
                  setWindow({ start: end - d, end });
                  setBack([]);
                }}
              >
                {!live && (
                  <option value="custom">
                    {timeLabel(window.start)} — {timeLabel(window.end)}
                  </option>
                )}
                <option value={900}>Last 15 minutes</option>
                <option value={3600}>Last hour</option>
                <option value={21600}>Last 6 hours</option>
                <option value={86400}>Last 24 hours</option>
              </select>
              <ChevronDown size={13} />
            </div>
            <button
              className="icon-button"
              title={live ? "Pause refresh · every 15s" : "Resume live refresh"}
              aria-label={live ? "Pause refresh" : "Resume live refresh"}
              onClick={() => {
                if (!live) refresh();
                setLive(!live);
              }}
            >
              {live ? <Pause size={15} /> : <Play size={15} />}
            </button>
            <button
              className="icon-button"
              title="Refresh to now"
              aria-label="Refresh to now"
              onClick={refresh}
            >
              <RefreshCw size={15} />
            </button>
          </div>
        </div>
      </header>
      <main>
        {namespaces.isError && (
          <div className="notice error">
            Namespace discovery failed: {namespaces.error.message}
          </div>
        )}
        {dashboards.isError && (
          <div className="notice error">
            Couldn’t load dashboards: {dashboards.error.message}
          </div>
        )}
        {dashboards.isPending && (
          <div className="dashboard-loading">Loading dashboard…</div>
        )}
        {dashboards.data?.length === 0 && (
          <div className="empty-workspace">
            <h2>No dashboards yet</h2>
            <p>Your agent can create one through the MCP endpoint.</p>
          </div>
        )}
        {dashboards.data && dashboards.data.length > 0 && !dashboard && (
          <div className="empty-workspace">
            <h2>Dashboard not found</h2>
            <p>Select an available dashboard above.</p>
          </div>
        )}
        {dashboard?.panels.length === 0 && (
          <div className="empty-workspace">
            <h2>This dashboard is empty</h2>
            <p>Ask your agent to add a chart.</p>
          </div>
        )}
        <div className="dashboard-grid">
          {dashboard?.panels.map((p) => (
            <PanelCard
              key={p.id}
              panel={p}
              namespace={namespace}
              window={window}
              onZoom={zoom}
              onExpand={() => setExpanded(p.id)}
            />
          ))}
        </div>
        <footer className="page-footer">
          Drag a chart to zoom · Shift-click a legend to isolate a series
        </footer>
      </main>
      {expandedPanel && (
        <div
          className="expand-backdrop"
          onMouseDown={(e) => {
            if (e.currentTarget === e.target) setExpanded(null);
          }}
        >
          <div
            className="expand-content"
            ref={expandedRef}
            onKeyDown={(e) => {
              if (e.key !== "Tab") return;
              const elements =
                expandedRef.current?.querySelectorAll<HTMLButtonElement>(
                  "button:not([disabled])",
                );
              if (!elements?.length) return;
              const first = elements[0],
                last = elements[elements.length - 1];
              if (e.shiftKey && document.activeElement === first) {
                e.preventDefault();
                last.focus();
              } else if (!e.shiftKey && document.activeElement === last) {
                e.preventDefault();
                first.focus();
              }
            }}
            role="dialog"
            aria-modal="true"
            aria-label={expandedPanel.title}
          >
            <PanelCard
              panel={expandedPanel}
              namespace={namespace}
              window={window}
              onZoom={zoom}
              onExpand={() => setExpanded(null)}
              expanded
            />
          </div>
        </div>
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
