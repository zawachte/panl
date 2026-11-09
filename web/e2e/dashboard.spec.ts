import { test, expect } from "@playwright/test";
test("read-only dashboard supports filtering, zoom, series controls and live agent updates", async ({
  page,
  request,
}) => {
  const errors: string[] = [];
  const mutations: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("request", (r) => {
    if (r.url().includes("/api/dashboards") && r.method() !== "GET")
      mutations.push(r.method() + " " + r.url());
  });
  await page.goto("/");
  await expect(page.getByRole("status")).toHaveText("Prometheus connected");
  await expect(page.locator(".dashboard-grid .panel")).toHaveCount(6);
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6, {
    timeout: 30000,
  });
  await expect(page.getByText("Couldn’t load this panel")).toHaveCount(0);
  await expect(
    page.locator('input,textarea,[contenteditable="true"]'),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: /inspect|edit|save|undo/i }),
  ).toHaveCount(0);
  await page.selectOption("#namespace", "checkout");
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6);
  await expect(page.locator(".legend-row").first()).toContainText("checkout");
  await page.screenshot({ path: "/tmp/panl-desktop.png", fullPage: true });
  const first = page.locator(".legend-row").first();
  await first.click();
  await expect(first).toHaveClass(/muted-series/);
  await first.click();
  await expect(first).not.toHaveClass(/muted-series/);
  const over = page.locator(".u-over").first();
  const box = (await over.boundingBox())!;
  await page.mouse.move(box.x + box.width * 0.25, box.y + box.height * 0.4);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.75, box.y + box.height * 0.4, {
    steps: 8,
  });
  await page.mouse.up();
  await expect(
    page.getByRole("button", { name: "Resume live refresh" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Previous time range" }).click();
  await page
    .getByRole("button", { name: "Expand CPU usage", exact: true })
    .click();
  await expect(page.locator(".expanded-panel")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator(".expanded-panel")).toHaveCount(0);
  // Configuration changes come from an external agent, never browser controls.
  const d = await (await request.get("/api/dashboards/kubernetes")).json();
  const panel = d.panels[0];
  const oldTitle = panel.title;
  panel.title = "Agent-updated CPU";
  const result = await request.patch("/api/dashboards/kubernetes/panels/cpu", {
    headers: { Authorization: "Bearer playwright-agent-token-for-local-tests" },
    data: { revision: d.revision, panel },
  });
  expect(result.ok()).toBeTruthy();
  try {
    await expect(
      page.getByRole("heading", { name: "Agent-updated CPU" }),
    ).toBeVisible({ timeout: 10000 });
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "Agent-updated CPU" }),
    ).toBeVisible();
  } finally {
    const fresh = await result.json();
    panel.title = oldTitle;
    const restored = await request.patch(
      "/api/dashboards/kubernetes/panels/cpu",
      {
        headers: {
          Authorization: "Bearer playwright-agent-token-for-local-tests",
        },
        data: { revision: fresh.revision, panel },
      },
    );
    expect(restored.ok()).toBeTruthy();
  }
  expect(errors).toEqual([]);
  expect(mutations).toEqual([]);
});
test("mobile reading layout has no editors or horizontal overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(page.locator(".dashboard-grid canvas")).toHaveCount(6, {
    timeout: 30000,
  });
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  await expect(
    page.locator('input,textarea,[contenteditable="true"]'),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: /inspect|edit|save|undo/i }),
  ).toHaveCount(0);
  await page.screenshot({ path: "/tmp/panl-mobile.png", fullPage: true });
});
