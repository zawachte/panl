import { useEffect, useMemo, useRef } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Maximize2, X } from "lucide-react";
import { api } from "./data";
import type { Panel, QueryResult, Window } from "./data";
import { Chart } from "./Chart";

type Frame = {
  results: QueryResult[];
  window: Window;
  namespace: string;
  queries: Panel["queries"];
};
export function PanelCard({
  panel,
  namespace,
  window,
  onZoom,
  onExpand,
  expanded = false,
}: {
  panel: Panel;
  namespace: string;
  window: Window;
  onZoom: (w: Window) => void;
  onExpand: () => void;
  expanded?: boolean;
}) {
  const previous = useRef<Frame | null>(null);
  const result = useQuery({
    queryKey: [
      "panel-metrics",
      panel.id,
      panel.queries,
      namespace,
      window.start,
      window.end,
    ],
    queryFn: async ({ signal }) => ({
      results: await Promise.all(
        panel.queries.map((q) =>
          api<QueryResult>(
            "/api/query",
            { expr: q.expr, namespace, ...window, points: 600 },
            "POST",
            signal,
          ),
        ),
      ),
      window,
      namespace,
      queries: panel.queries,
    }),
    placeholderData: keepPreviousData,
  });
  useEffect(() => {
    if (result.data && !result.isPlaceholderData)
      previous.current = result.data;
  }, [result.data, result.isPlaceholderData]);
  const frame = result.data ?? previous.current;
  const series = useMemo(
    () =>
      frame?.results.flatMap((r, i) =>
        r.series.map((s) => ({ series: s, query: frame.queries[i] })),
      ) ?? [],
    [frame],
  );
  const warnings = frame?.results.flatMap((r) => r.warnings ?? []) ?? [];
  const hasSamples = series.some((s) =>
    s.series.points.some((p) => p.value !== null),
  );
  const stale = !!result.error && !!frame;
  return (
    <section
      className={`panel ${expanded ? "expanded-panel" : ""}`}
      aria-label={panel.title}
      aria-busy={result.isFetching}
    >
      <div className="panel-heading">
        <h2>{panel.title}</h2>
        <div className="panel-actions">
          {stale ? (
            <span
              className="stale-label"
              role="status"
              title={result.error?.message}
            >
              Previous data · refresh failed
            </span>
          ) : (
            result.isFetching && (
              <span className="fetch-dot" title="Updating metrics" />
            )
          )}
          <button
            className="icon-button"
            aria-label={
              expanded ? "Close expanded chart" : `Expand ${panel.title}`
            }
            title={expanded ? "Close" : "Expand chart"}
            onClick={onExpand}
          >
            {expanded ? <X size={15} /> : <Maximize2 size={14} />}
          </button>
        </div>
      </div>
      {hasSamples && frame ? (
        <Chart
          series={series}
          unit={panel.unit}
          window={frame.window}
          onZoom={onZoom}
          height={expanded ? 365 : 207}
        />
      ) : result.error ? (
        <div className="panel-message error">
          <span>Couldn’t load this panel</span>
          <small>{result.error.message}</small>
        </div>
      ) : result.isPending ? (
        <div className="panel-message loading">Loading metrics…</div>
      ) : (
        <div className="panel-message">
          <span>No samples in this interval</span>
          <small>Try a wider time range or another namespace.</small>
        </div>
      )}
      {warnings.length > 0 && (
        <details className="warning">
          <summary>Query warning</summary>
          {warnings.map((w, i) => (
            <p key={i}>{w}</p>
          ))}
        </details>
      )}
    </section>
  );
}
