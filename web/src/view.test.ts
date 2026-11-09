import { expect, it } from "vitest";
import { readView, viewSearch } from "./view";
it("restores shared absolute bounds, namespace and dashboard", () => {
  const v = readView("?dashboard=checkout&namespace=prod&from=1000&to=2000");
  expect(v.live).toBe(false);
  expect(v.window).toEqual({ start: 1000, end: 2000 });
  expect(readView(viewSearch(v))).toEqual(v);
});
it("bounds invalid URLs and keeps live links relative", () => {
  const v = readView("?range=-1&from=0&to=999999999", 5000);
  expect(v.window).toEqual({ start: 1400, end: 5000 });
  expect(v.live).toBe(true);
  expect(viewSearch(v)).not.toContain("from=");
});
