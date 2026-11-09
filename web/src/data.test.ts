import { describe, it, expect } from "vitest";
import { alignSeries, formatValue, seriesName, colorFor } from "./data";
describe("metric presentation", () => {
  it("aligns sparse timestamps without turning gaps into zeros", () => {
    expect(
      alignSeries([
        {
          labels: {},
          points: [
            { time: 1, value: 0 },
            { time: 3, value: 2 },
          ],
        },
        {
          labels: {},
          points: [
            { time: 2, value: 4 },
            { time: 3, value: null },
          ],
        },
      ]),
    ).toEqual([
      [1, 2, 3],
      [0, null, 2],
      [null, 4, null],
    ]);
  });
  it("formats units and distinguishes missing data", () => {
    expect(formatValue(null, "bytes")).toBe("—");
    expect(formatValue(0, "count")).toBe("0");
    expect(formatValue(1024 ** 3, "bytes")).toBe("1 GiB");
    expect(formatValue(0.125, "cores")).toBe("125 mCPU");
    expect(formatValue(50, "percent")).toBe("50%");
  });
  it("keeps series identity and color stable", () => {
    expect(seriesName({ namespace: "monitoring" }, "{{namespace}}")).toBe(
      "monitoring",
    );
    expect(colorFor("monitoring")).toBe(colorFor("monitoring"));
    expect(seriesName({}, "{{pod}}")).toBe("value");
  });
});
