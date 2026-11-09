export type PanelQuery = {
  expr: string;
  legend: string;
  kind?: "request" | "limit" | "";
};
export type Panel = {
  id: string;
  title: string;
  unit: string;
  queries: PanelQuery[];
};
export type Dashboard = {
  id: string;
  title: string;
  description: string;
  revision: number;
  panels: Panel[];
};
export type MetricSeries = {
  labels: Record<string, string>;
  points: { time: number; value: number | null }[];
};
export type QueryResult = {
  series: MetricSeries[];
  warnings: string[] | null;
  expr: string;
};
export type Window = { start: number; end: number };
export async function api<T>(
  path: string,
  body?: unknown,
  method = "POST",
  signal?: AbortSignal,
): Promise<T> {
  const r = await fetch(path, {
    method: body === undefined ? "GET" : method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || `Request failed (${r.status})`);
  return data;
}
export function seriesName(labels: Record<string, string>, template: string) {
  const name = template.replace(
    /\{\{([^}]+)\}\}/g,
    (_, key: string) => labels[key.trim()] ?? "",
  );
  return (
    name.trim() ||
    Object.entries(labels)
      .filter(([k]) => k !== "__name__")
      .map(([k, v]) => `${k}=${v}`)
      .join(", ") ||
    labels.__name__ ||
    "value"
  );
}
export const palette = [
  "#79c9b2",
  "#93b6f5",
  "#d9b77b",
  "#b6a0df",
  "#db929e",
  "#88c8d5",
  "#b7c98b",
  "#d6a489",
];
export function colorFor(name: string) {
  let hash = 0;
  for (const c of name) hash = ((hash << 5) - hash + c.charCodeAt(0)) | 0;
  return palette[(hash >>> 0) % palette.length];
}
export function formatValue(value: number | null | undefined, unit: string) {
  if (value == null || !Number.isFinite(value)) return "—";
  if (unit === "bytes" || unit === "bytes/s") {
    const units =
      unit === "bytes"
        ? ["B", "KiB", "MiB", "GiB", "TiB"]
        : ["B/s", "KiB/s", "MiB/s", "GiB/s", "TiB/s"];
    let i = 0;
    while (Math.abs(value) >= 1024 && i < units.length - 1) {
      value /= 1024;
      i++;
    }
    return `${value.toLocaleString(undefined, { maximumFractionDigits: 1 })} ${units[i]}`;
  }
  if (unit === "cores" && Math.abs(value) < 1)
    return `${(value * 1000).toLocaleString(undefined, { maximumFractionDigits: 1 })} mCPU`;
  return `${value.toLocaleString(undefined, { maximumFractionDigits: unit === "count" ? 1 : 2 })}${unit === "percent" ? "%" : unit === "cores" ? " cores" : unit === "seconds" ? " s" : unit === "requests/s" ? " req/s" : ""}`;
}
export function alignSeries(
  series: MetricSeries[],
): [number[], ...(number | null)[][]] {
  const times = [
    ...new Set(series.flatMap((s) => s.points.map((p) => p.time))),
  ].sort((a, b) => a - b);
  return [
    times,
    ...series.map((s) => {
      const values = new Map(s.points.map((p) => [p.time, p.value]));
      return times.map((t) => values.get(t) ?? null);
    }),
  ];
}
