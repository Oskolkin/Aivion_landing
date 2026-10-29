import { expect, test, type Page } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";

const funnelReady = {
  schema_version: "funnel_projection_v2",
  state: "READY",
  status: "READY",
  period: { current: { key: "current_7d" }, previous: { key: "previous_7d" } },
  stages: [
    { key: "PDP_IMPRESSIONS", status: "READY", amount: "100" },
    { key: "PDP_CART_ADDS", status: "READY", amount: "10" },
  ],
  transitions: [],
  insight: { headline: "Воронка", statement: "Цепочка совместима", primary_domain: "FUNNEL" },
  related_action_local_refs: ["a1"],
};

function homeFor(pub: { id: string; amount: string; units: string; stockTitle: string; campaign: string; funnel?: Record<string, unknown> }) {
  return {
    state: "READY",
    publication: {
      id: pub.id,
      analysis_date: "2026-08-31",
      analysis_cutoff_at: "2026-08-31T21:00:00Z",
      published_at: "2026-08-31T22:00:00Z",
    },
    overview: {
      schema_version: "overview_projection_v2",
      seller_account_id: 1,
      analysis_date: "2026-08-31",
      analysis_cutoff_at: "2026-08-31T21:00:00Z",
      kpis: [
        { key: "ORDERED_AMOUNT", status: "READY", amount: pub.amount, unit: "RUB" },
        { key: "ORDERED_UNITS", status: "READY", amount: pub.units, unit: "UNITS" },
        { key: "AD_SHARE", status: "READY", amount: "0.12", unit: "RATIO" },
      ],
      executive_headline: `Сводка ${pub.id}`,
      executive_summary: `Сводка ${pub.id}`,
      takeaways: [{ statement: "Остатки требуют внимания", primary_domain: "STOCK" }],
      findings: [{ title: pub.stockTitle, statement: "Известный ноль", primary_domain: "STOCK", overview_bucket: "ACTIVE_RISK" }],
    },
    funnel: pub.funnel ?? funnelReady,
    stock: {
      schema_version: "stock_projection_v2",
      coverage_bands: [
        { key: "out_of_stock", count: 2, state: "KNOWN" },
        { key: "coverage_0_3", count: 1, state: "KNOWN" },
        { key: "coverage_4_7", count: 0, state: "KNOWN" },
        { key: "coverage_8_14", count: 0, state: "KNOWN" },
      ],
      findings: [{ title: "Пополнить", statement: "", primary_domain: "STOCK", overview_bucket: "NONE" }],
    },
    advertising: {
      schema_version: "advertising_projection_v2",
      account_ad_spend_share: { key: "account_ad_spend_share", status: "READY", amount: "0.12" },
      yesterday_ad_spend_share: { key: "yesterday_account_ad_spend_share", status: "MISSING" },
    },
    focus: [
      {
        action_view_id: `view-${pub.id}`,
        display_order: 1,
        display_title: "Пополнить 4 товара",
        display_summary: "",
        action_type: "SKU_INCREASE_REPLENISHMENT",
        family: "INVENTORY",
        active_member_count: 4,
      },
    ],
  };
}

const P1 = { id: "101", amount: "120000", units: "40", stockTitle: "Остаток SKU P1", campaign: "Camp-P1" };
const P2 = { id: "202", amount: "99000", units: "33", stockTitle: "Остаток SKU P2", campaign: "Camp-P2" };

async function noPageOverflow(page: Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(2);
}

async function stubDashboard(page: Page, home: ReturnType<typeof homeFor>, opts?: { delayMs?: number; adsUnavailable?: boolean }) {
  await stubOzonConnectionReady(page);
  await page.route("**/api/v1/dashboard/home", async (route) => {
    if (opts?.delayMs) await new Promise((r) => setTimeout(r, opts.delayMs));
    const body = opts?.adsUnavailable
      ? {
          ...home,
          advertising: {
            schema_version: "advertising_projection_v2",
            account_ad_spend_share: { key: "account_ad_spend_share", status: "MISSING" },
            yesterday_ad_spend_share: { key: "yesterday_account_ad_spend_share", status: "MISSING" },
          },
        }
      : home;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
  await page.route("**/api/v1/dashboard/stock/skus**", async (route) => {
    const url = new URL(route.request().url());
    const publicationId = url.searchParams.get("publication_id") ?? home.publication.id;
    const sku = publicationId === P2.id ? "p2-sku" : "p1-sku";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        publication_id: publicationId,
        items: [
          { sku_id: sku, sellable_units: 0, sellable_units_state: "KNOWN", coverage_band: "out_of_stock" },
          { sku_id: `${sku}-unknown`, sellable_units_state: "UNKNOWN", sellable_units_reason: "INVENTORY_POOL_OVERLAP_UNVERIFIED", known_subtotal_units: 40 },
        ],
        has_more: false,
      }),
    });
  });
  await page.route("**/api/v1/dashboard/advertising/campaigns**", async (route) => {
    const url = new URL(route.request().url());
    const publicationId = url.searchParams.get("publication_id") ?? home.publication.id;
    const title = publicationId === P2.id ? P2.campaign : P1.campaign;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        publication_id: publicationId,
        items: [
          {
            campaign_id: publicationId,
            title,
            budget_state: "KNOWN",
            budget_amount: "0",
            paid_spend_state: "UNKNOWN",
            attributed_order_amount_state: "UNKNOWN",
            campaign_drr_state: "UNKNOWN",
          },
        ],
        has_more: false,
      }),
    });
  });
}

async function stubActions(page: Page) {
  let page1 = true;
  await page.route("**/api/v1/actions?**", async (route) => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("cursor")) page1 = false;
    const groups = page1
      ? [
          {
            action_view_id: "view1",
            display_order: 1,
            display_title: "Пополнить 4 товара",
            display_summary: "Пополнить запас",
            action_type: "SKU_INCREASE_REPLENISHMENT",
            family: "INVENTORY",
            presentation_template_key: "SKU_INCREASE_REPLENISHMENT",
            active_member_count: 4,
            members: [
              { case_id: "c1", member_order: 1, action_type: "SKU_INCREASE_REPLENISHMENT", target_kind: "SKU", target_id: "1", user_state: "OPEN", system_state: "ACTIVE", active: true, parameters: { recommended_units: "8" } },
            ],
          },
        ]
      : [
          {
            action_view_id: "view2",
            display_order: 2,
            display_title: "Вторая страница",
            display_summary: "",
            action_type: "SKU_ADJUST_PRICE",
            family: "PRICING",
            presentation_template_key: "SKU_ADJUST_PRICE",
            active_member_count: 1,
            members: [
              { case_id: "c2", member_order: 1, action_type: "SKU_ADJUST_PRICE", target_kind: "SKU", target_id: "9", user_state: "OPEN", system_state: "ACTIVE", active: true, parameters: { current_price: "100", recommended_price: "90" } },
            ],
          },
        ];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        publication_id: "101",
        state: "READY",
        groups,
        has_more: page1,
        next_cursor: page1 ? "cursor-page-2" : "",
      }),
    });
  });
  await page.route("**/api/v1/actions/cases/**", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true }) });
  });
}

test("prompt14 overview kpis desktop", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await stubActions(page);
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("overview-kpi-grid")).toBeVisible({ timeout: 45_000 });
  await expect(page.getByTestId("overview-kpi-ORDERED_AMOUNT")).toBeVisible();
  await expect(page.getByTestId("overview-kpi-ORDERED_UNITS")).toBeVisible();
  await expect(page.getByTestId("overview-kpi-AD_SHARE")).toBeVisible();
  await expect(page.getByText("Заказано на сумму")).toBeVisible();
  await expect(page.getByText("Заказано товаров")).toBeVisible();
  await expect(page.getByText("Доля рекламы")).toBeVisible();
  await expect(page.getByText("Средний чек")).toHaveCount(0);
  await expect(page.getByText("AOV", { exact: true })).toHaveCount(0);
  await noPageOverflow(page);
});

test("prompt14 funnel READY", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("dashboard-funnel-page")).toBeVisible({ timeout: 45_000 });
  await expect(page.getByTestId("funnel-visualization")).toBeVisible();
  await expect(page.getByText("Средний чек")).toHaveCount(0);
  await noPageOverflow(page);
});

test("prompt14 funnel PARTIAL", async ({ page }) => {
  await stubDashboard(page, homeFor({ ...P1, funnel: { ...funnelReady, state: "PARTIAL", status: "PARTIAL" } }));
  await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("funnel-partial")).toBeVisible({ timeout: 45_000 });
});

test("prompt14 funnel READY_EMPTY", async ({ page }) => {
  await stubDashboard(page, homeFor({ ...P1, funnel: { ...funnelReady, state: "READY_EMPTY", status: "READY_EMPTY", stages: [] } }));
  await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("funnel-ready-empty")).toBeVisible({ timeout: 45_000 });
});

test("prompt14 funnel UNAVAILABLE", async ({ page }) => {
  await stubDashboard(page, homeFor({ ...P1, funnel: { ...funnelReady, state: "UNAVAILABLE", status: "UNAVAILABLE", reason_code: "SUBSCRIPTION_REQUIRED", stages: [] } }));
  await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("funnel-unavailable")).toBeVisible({ timeout: 45_000 });
});

test("prompt14 funnel FAILED", async ({ page }) => {
  await stubDashboard(page, homeFor({ ...P1, funnel: { ...funnelReady, state: "FAILED", status: "FAILED", stages: [] } }));
  await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("funnel-failed")).toBeVisible({ timeout: 45_000 });
});

test("prompt14 stock unknown vs zero filters lazy", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto("/app/dashboard/stock", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("dashboard-stock-page")).toBeVisible({ timeout: 45_000 });
  await expect(page.getByRole("button", { name: "Нет остатка" })).toBeVisible();
  await expect(page.getByTestId("stock-sku-table")).toBeVisible();
  await expect(page.getByText("p1-sku", { exact: true })).toBeVisible();
  await expect(page.getByText("—").first()).toBeVisible();
  await page.getByTestId("stock-filters").getByRole("button", { name: "0–3 дня" }).click();
  await noPageOverflow(page);
});

test("prompt14 advertising unavailable and campaign attribution", async ({ page }) => {
  await stubDashboard(page, homeFor(P1), { adsUnavailable: true });
  await page.goto("/app/dashboard/advertising", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("dashboard-advertising-page")).toBeVisible({ timeout: 45_000 });
  await expect(page.getByTestId("account-ad-spend-share")).toHaveText("—");
  await expect(page.getByText("Кампании для проверки")).toBeVisible();
  await expect(page.getByText("не Canonical Sales")).toBeVisible();
});

test("prompt14 advertising known zero budget", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await page.goto("/app/dashboard/advertising", { waitUntil: "domcontentloaded" });
  await expect(page.getByText("Camp-P1")).toBeVisible({ timeout: 45_000 });
  await expect(page.getByText("0 ₽")).toBeVisible();
});

test("prompt14 actions group done dismiss pagination filters", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await stubActions(page);
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("actions-workspace")).toBeVisible({ timeout: 45_000 });
  const list = page.getByTestId("actions-list").filter({ visible: true });
  await expect(list.getByText("Пополнить 4 товара")).toBeVisible();
  await list.getByRole("button", { name: /Пополнить 4 товара/ }).click();
  await expect(page.getByTestId("actions-detail").filter({ visible: true })).toBeVisible();
  await page.getByRole("button", { name: "Сделано" }).filter({ visible: true }).click();
  await page.getByRole("button", { name: "Неактуально" }).filter({ visible: true }).click();
  await page.getByRole("button", { name: "Показать ещё действия" }).filter({ visible: true }).click();
  await expect(list.getByText("Вторая страница")).toBeVisible();
  await page.getByRole("tab", { name: "Выполненные" }).filter({ visible: true }).click();
  await page.getByRole("button", { name: "Остатки" }).filter({ visible: true }).click();
  await expect(page.getByLabel("Поиск действий").filter({ visible: true })).toBeVisible();
});

test("prompt14 P1 to P2 publication consistency", async ({ page }) => {
  let current = "p1";
  await stubOzonConnectionReady(page);
  await page.route("**/api/v1/dashboard/home", async (route) => {
    const pub = current === "p1" ? P1 : P2;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(homeFor(pub)) });
  });
  await page.route("**/api/v1/dashboard/stock/skus**", async (route) => {
    const publicationId = new URL(route.request().url()).searchParams.get("publication_id");
    const sku = publicationId === P2.id ? "p2-sku" : "p1-sku";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ publication_id: publicationId, items: [{ sku_id: sku, sellable_units: 0, sellable_units_state: "KNOWN" }], has_more: false }),
    });
  });
  await page.route("**/api/v1/dashboard/advertising/campaigns**", async (route) => {
    const publicationId = new URL(route.request().url()).searchParams.get("publication_id");
    const title = publicationId === P2.id ? P2.campaign : P1.campaign;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ publication_id: publicationId, items: [{ campaign_id: publicationId, title, budget_state: "UNKNOWN" }], has_more: false }),
    });
  });
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("overview-kpi-ORDERED_AMOUNT")).toContainText("120");
  await page.getByTestId("dashboard-tab-stock").click();
  await expect(page.getByText("p1-sku")).toBeVisible({ timeout: 45_000 });
  await page.getByTestId("dashboard-tab-advertising").click();
  await expect(page.getByText("Camp-P1")).toBeVisible();
  current = "p2";
  await page.getByRole("button", { name: "Повторить" }).count();
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("overview-kpi-ORDERED_AMOUNT")).toContainText("99");
  await page.getByTestId("dashboard-tab-stock").click();
  await expect(page.getByText("p2-sku")).toBeVisible({ timeout: 45_000 });
  await expect(page.getByText("p1-sku")).toHaveCount(0);
  await page.getByTestId("dashboard-tab-advertising").click();
  await expect(page.getByText("Camp-P2")).toBeVisible();
  await expect(page.getByText("Camp-P1")).toHaveCount(0);
});

test("prompt14 lazy pinning keeps P1 stock after current switches to P2", async ({ page }) => {
  let current = "p1";
  const stockPubs: string[] = [];
  await stubOzonConnectionReady(page);
  await page.route("**/api/v1/dashboard/home", async (route) => {
    const pub = current === "p1" ? P1 : P2;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(homeFor(pub)) });
  });
  await page.route("**/api/v1/dashboard/stock/skus**", async (route) => {
    const publicationId = new URL(route.request().url()).searchParams.get("publication_id") ?? "";
    stockPubs.push(publicationId);
    const sku = publicationId === P2.id ? "p2-sku" : "p1-sku";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ publication_id: publicationId, items: [{ sku_id: sku, sellable_units: 0, sellable_units_state: "KNOWN" }], has_more: false }),
    });
  });
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("overview-kpi-grid")).toBeVisible({ timeout: 45_000 });
  current = "p2";
  await page.getByTestId("dashboard-tab-stock").click();
  await expect(page.getByText("p1-sku")).toBeVisible({ timeout: 45_000 });
  expect(stockPubs[0]).toBe(P1.id);
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("overview-kpi-ORDERED_AMOUNT")).toContainText("99");
  await page.getByTestId("dashboard-tab-stock").click();
  await expect(page.getByText("p2-sku")).toBeVisible({ timeout: 45_000 });
  expect(stockPubs.at(-1)).toBe(P2.id);
});

test("prompt14 mobile tablet desktop overflow", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await stubActions(page);
  for (const width of [390, 768, 1400]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("overview-kpi-grid")).toBeVisible({ timeout: 45_000 });
    await noPageOverflow(page);
    await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("dashboard-funnel-page")).toBeVisible();
    await noPageOverflow(page);
    await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("actions-workspace")).toBeVisible();
    if (width < 1280) {
      await page.getByRole("button", { name: /Пополнить 4 товара/ }).filter({ visible: true }).click();
      await expect(page.getByRole("button", { name: "Назад к действиям" }).filter({ visible: true })).toBeVisible();
      await page.getByRole("button", { name: "Назад к действиям" }).filter({ visible: true }).click();
      await expect(page.getByTestId("actions-list").filter({ visible: true })).toBeVisible();
    }
    await noPageOverflow(page);
  }
});

test("prompt14 dashboard loading preanalysis failed", async ({ page }) => {
  await stubOzonConnectionReady(page);
  await page.route("**/api/v1/dashboard/home", async (route) => {
    await new Promise((r) => setTimeout(r, 800));
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ state: "PRE_ANALYSIS" }) });
  });
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("dashboard-skeleton")).toBeVisible();
  await expect(page.getByText("Анализ ещё не опубликован")).toBeVisible({ timeout: 45_000 });

  await page.unroute("**/api/v1/dashboard/home");
  await page.route("**/api/v1/dashboard/home", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ state: "FAILED", error: { message_key: "publication.failed" } }) });
  });
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByText("Публикация недоступна")).toBeVisible({ timeout: 45_000 });
});

test("prompt14 actions empty and api failure", async ({ page }) => {
  await stubOzonConnectionReady(page);
  await page.route("**/api/v1/actions?**", async (route) => {
    await route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "boom" }) });
  });
  await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("actions-workspace").getByRole("alert")).toBeVisible({ timeout: 45_000 });

  await page.unroute("**/api/v1/actions?**");
  await page.route("**/api/v1/actions?**", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ state: "READY", groups: [], history: [] }) });
  });
  await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("actions-empty").filter({ visible: true })).toBeVisible({ timeout: 45_000 });
});

test("prompt14 actions mutation failure", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await page.route("**/api/v1/actions?**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "READY",
        groups: [
          {
            action_view_id: "view1",
            display_title: "Пополнить 4 товара",
            display_summary: "",
            action_type: "SKU_INCREASE_REPLENISHMENT",
            family: "INVENTORY",
            presentation_template_key: "SKU_INCREASE_REPLENISHMENT",
            active_member_count: 1,
            members: [{ case_id: "c1", member_order: 1, action_type: "SKU_INCREASE_REPLENISHMENT", target_kind: "SKU", target_id: "1", user_state: "OPEN", system_state: "ACTIVE", active: true }],
          },
        ],
      }),
    });
  });
  await page.route("**/api/v1/actions/cases/**", async (route) => {
    await route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "fail" }) });
  });
  await page.setViewportSize({ width: 1400, height: 900 });
  await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: /Пополнить 4 товара/ }).filter({ visible: true }).click();
  await page.getByRole("button", { name: "Сделано" }).filter({ visible: true }).click();
  await expect(page.getByTestId("actions-mutation-error")).toBeVisible();
});

test("prompt14 accessibility labels", async ({ page }) => {
  await stubDashboard(page, homeFor(P1));
  await stubActions(page);
  await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByLabel("Поиск действий").filter({ visible: true })).toBeVisible({ timeout: 45_000 });
  await expect(page.getByRole("tablist", { name: "Статус действий" }).filter({ visible: true })).toBeVisible();
  await page.keyboard.press("Tab");
});
