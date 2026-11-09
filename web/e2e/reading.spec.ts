import { expect, test } from "@playwright/test";
test("a paused shared URL restores its dashboard, namespace and exact interval", async ({
  page,
}) => {
  const end = Math.floor(Date.now() / 1000);
  const start = end - 1800;
  await page.goto(
    `/?dashboard=kubernetes&namespace=checkout&from=${start}&to=${end}`,
  );
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6);
  await expect(page.getByLabel("Namespace", { exact: true })).toHaveValue(
    "checkout",
  );
  await expect(
    page.getByRole("button", { name: "Resume live refresh" }),
  ).toBeVisible();
  await page.reload();
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6);
  const url = new URL(page.url());
  expect(url.searchParams.get("from")).toBe(String(start));
  expect(url.searchParams.get("to")).toBe(String(end));
});
test("refresh retains the canvas and hidden series; failures show previous data explicitly", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6);
  await page
    .getByRole("button", { name: "Pause refresh", exact: true })
    .click();
  const legend = page.locator(".legend-row").first();
  await legend.click();
  await expect(legend).toHaveClass(/muted-series/);
  await page.evaluate(() => {
    (
      globalThis as typeof globalThis & { originalCanvas?: Element | null }
    ).originalCanvas = document.querySelector("canvas");
  });
  await page.getByRole("button", { name: "Refresh to now" }).click();
  await expect(page.locator(".dashboard-grid .panel").first()).toHaveAttribute(
    "aria-busy",
    "false",
  );
  expect(
    await page.evaluate(
      () =>
        document.querySelector("canvas") ===
        (globalThis as typeof globalThis & { originalCanvas?: Element | null })
          .originalCanvas,
    ),
  ).toBe(true);
  await expect(legend).toHaveClass(/muted-series/);
  await page.route("**/api/query", (r) =>
    r.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "Prometheus unavailable" }),
    }),
  );
  await page.getByRole("button", { name: "Refresh to now" }).click();
  await expect(page.getByText("Previous data · refresh failed")).toHaveCount(
    6,
    { timeout: 15000 },
  );
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6);
});
test("100-series panels remain usable and do not overflow", async ({
  page,
}) => {
  const end = Math.floor(Date.now() / 1000);
  const points = Array.from({ length: 300 }, (_, i) => ({
    time: end - 3600 + i * 12,
    value: 0.1 + Math.sin(i / 15) * 0.02,
  }));
  await page.route("**/api/query", (r) =>
    r.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        series: Array.from({ length: 100 }, (_, i) => ({
          labels: { namespace: `namespace-${i}` },
          points,
        })),
        warnings: [],
        expr: "synthetic browser load test",
      }),
    }),
  );
  const start = Date.now();
  await page.goto("/");
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6, {
    timeout: 15000,
  });
  await expect(page.locator(".legend-row")).toHaveCount(600);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(1440);
  console.log(
    `Six panels × 100 series × 300 points rendered in ${Date.now() - start} ms on this runner`,
  );
});
