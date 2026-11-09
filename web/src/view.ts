import type { Window } from "./data";
export type ViewState = {
  dashboardId: string;
  namespace: string;
  duration: number;
  live: boolean;
  window: Window;
};
export function readView(
  search: string,
  now = Math.floor(Date.now() / 1000),
): ViewState {
  const params = new URLSearchParams(search);
  const range = Number(params.get("range") || 3600);
  const duration = [900, 3600, 21600, 86400].includes(range) ? range : 3600;
  const start = Number(params.get("from"));
  const end = Number(params.get("to"));
  const absolute =
    params.has("from") &&
    params.has("to") &&
    Number.isFinite(start) &&
    Number.isFinite(end) &&
    start >= 0 &&
    end > start &&
    end - start <= 7 * 86400;
  return {
    dashboardId: params.get("dashboard") || "kubernetes",
    namespace: params.get("namespace") || "",
    duration,
    live: absolute ? false : params.get("live") !== "0",
    window: absolute ? { start, end } : { start: now - duration, end: now },
  };
}
export function viewSearch(state: ViewState): string {
  const p = new URLSearchParams();
  p.set("dashboard", state.dashboardId);
  if (state.namespace) p.set("namespace", state.namespace);
  p.set("range", String(state.duration));
  if (!state.live) {
    p.set("live", "0");
    p.set("from", String(state.window.start));
    p.set("to", String(state.window.end));
  }
  return "?" + p.toString();
}
