import { useEffect, useMemo, useRef, useState } from "react";
import uPlot from "uplot";
import { alignSeries, colorFor, formatValue, seriesName } from "./data";
import type { MetricSeries, PanelQuery, Window } from "./data";
import "uplot/dist/uPlot.min.css";
export type ChartSeries = { series: MetricSeries; query: PanelQuery };
export function Chart({
  series,
  unit,
  window,
  onZoom,
  height = 207,
}: {
  height?: number;
  series: ChartSeries[];
  unit: string;
  window: Window;
  onZoom: (w: Window) => void;
}) {
  const host = useRef<HTMLDivElement>(null);
  const plot = useRef<uPlot | null>(null);
  const zoom = useRef(onZoom);
  zoom.current = onZoom;
  const hidden = useRef(new Set<string>());
  const [index, setIndex] = useState<number | null>(null);
  const [hiddenNames, setHiddenNames] = useState(new Set<string>());
  const names = useMemo(
    () => series.map((s) => seriesName(s.series.labels, s.query.legend)),
    [series],
  );
  const keys = useMemo(
    () =>
      series.map((s) =>
        JSON.stringify([
          Object.entries(s.series.labels).sort(([a], [b]) =>
            a.localeCompare(b),
          ),
          s.query.expr,
        ]),
      ),
    [series],
  );
  const schema = JSON.stringify(
    series.map((s, i) => [keys[i], names[i], s.query.kind]),
  );
  const bounds = useRef(window);
  bounds.current = window;
  const latest = useRef({ series, names, keys });
  latest.current = { series, names, keys };
  const colors = useMemo(
    () =>
      series.map((s, i) =>
        colorFor(
          s.series.labels.pod
            ? `${s.series.labels.namespace ?? ""}/${s.series.labels.pod}`
            : (s.series.labels.namespace ?? names[i]),
        ),
      ),
    [series, names],
  );
  const colorValues = useRef(colors);
  colorValues.current = colors;
  const data = useMemo(
    () => alignSeries(series.map((s) => s.series)),
    [series],
  );
  useEffect(() => {
    if (!host.current) return;
    setIndex(null);
    const { series, names, keys } = latest.current;
    const p = new uPlot(
      {
        width: Math.max(200, host.current.clientWidth),
        height,
        padding: [12, 14, 0, 0],
        tzDate: (ts) => new Date(ts * 1000),
        legend: { show: false },
        scales: {
          x: {
            time: true,
            range: () => [bounds.current.start, bounds.current.end],
          },
          y: {
            range: (_u, min, max) => {
              if (!Number.isFinite(min) || !Number.isFinite(max)) return [0, 1];
              const pad =
                Math.max(max - min, Math.abs(max) * 0.1, 0.001) * 0.12;
              return [
                min >= 0 ? 0 : min - pad,
                max === 0 && min === 0 ? 1 : max + pad,
              ];
            },
          },
        },
        axes: [
          {
            stroke: "#77808d",
            grid: { show: false },
            ticks: { show: false },
            font: "11px system-ui",
            size: 30,
            gap: 9,
            space: 75,
            values: (_u, ticks) =>
              ticks.map((t) =>
                new Date(t * 1000).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                }),
              ),
          },
          {
            stroke: "#77808d",
            grid: { stroke: "#252a31", width: 1 },
            ticks: { show: false },
            font: "10px system-ui",
            size: 67,
            gap: 9,
            values: (_u, ticks) => ticks.map((v) => formatValue(v, unit)),
          },
        ],
        cursor: {
          sync: { key: "panl-time", scales: ["x", null] },
          drag: { x: true, y: false, setScale: false },
        },
        select: { show: true, left: 0, top: 0, width: 0, height: 0 },
        series: [
          {},
          ...series.map((s, i) => ({
            label: names[i],
            stroke: colorValues.current[i],
            width: s.query.kind ? 1 : 1.5,
            dash:
              s.query.kind === "request"
                ? [2, 3]
                : s.query.kind === "limit"
                  ? [6, 3]
                  : undefined,
            fill: undefined,
            show: !hidden.current.has(keys[i]),
            spanGaps: false,
            points: { show: false },
          })),
        ],
        hooks: {
          setSelect: [
            (u) => {
              if (u.select.width < 5) return;
              const start = Math.floor(u.posToVal(u.select.left, "x"));
              const end = Math.ceil(
                u.posToVal(u.select.left + u.select.width, "x"),
              );
              u.setSelect({ left: 0, top: 0, width: 0, height: 0 }, false);
              if (end > start) zoom.current({ start, end });
            },
          ],
          setCursor: [(u) => setIndex(u.cursor.idx ?? null)],
        },
      },
      data,
      host.current,
    );
    plot.current = p;
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width;
      if (width && width > 0)
        p.setSize({ width: Math.max(200, width), height });
    });
    observer.observe(host.current);
    return () => {
      observer.disconnect();
      p.destroy();
      plot.current = null;
    };
  }, [schema, unit, height]);
  useEffect(() => {
    const p = plot.current;
    if (!p) return;
    p.batch(() => {
      p.setData(data);
      p.setScale("x", { min: window.start, max: window.end });
    });
    setIndex(null);
  }, [data, window.start, window.end]);
  function toggle(i: number, solo: boolean) {
    const next = new Set(hidden.current);
    if (solo) {
      const others = keys.filter((_, j) => j !== i);
      const already = others.every((n) => next.has(n)) && !next.has(keys[i]);
      next.clear();
      if (!already) others.forEach((n) => next.add(n));
    } else {
      if (next.has(keys[i])) next.delete(keys[i]);
      else next.add(keys[i]);
    }
    hidden.current = next;
    setHiddenNames(new Set(next));
    keys.forEach((name, j) =>
      plot.current?.setSeries(j + 1, { show: !next.has(name) }),
    );
  }
  return (
    <>
      <div
        className="plot"
        ref={host}
        aria-label={`${unit} time-series chart`}
      />
      <div className="hover-time" aria-live="off">
        {index !== null && data[0][index] !== undefined
          ? new Date(data[0][index] * 1000).toLocaleString()
          : ""}
      </div>
      <div className="legend" aria-label="Chart series">
        {series.map((s, i) => {
          const points = data[i + 1];
          const value = index === null ? points?.at(-1) : points?.[index];
          return (
            <button
              key={keys[i]}
              className={`legend-row ${hiddenNames.has(keys[i]) ? "muted-series" : ""}`}
              onClick={(e) => toggle(i, e.shiftKey)}
              title={`${names[i]} · click to toggle, Shift+click to isolate`}
            >
              <span
                className="swatch"
                style={{
                  background: s.query.kind ? "transparent" : colors[i],
                  borderTop: s.query.kind
                    ? `2px dashed ${colors[i]}`
                    : undefined,
                }}
              />
              <span className="series-name">{names[i]}</span>
              <span className="series-value">{formatValue(value, unit)}</span>
            </button>
          );
        })}
      </div>
    </>
  );
}
