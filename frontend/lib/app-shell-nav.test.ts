import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

function extractSellerNavBlock(source: string): string {
  const start = source.indexOf("const SELLER_NAV");
  assert.ok(start >= 0, "SELLER_NAV config not found");
  const end = source.indexOf("const ADMIN_ONLY_NAV", start);
  assert.ok(end > start, "ADMIN_ONLY_NAV config not found");
  return source.slice(start, end);
}

function run() {
  const shellPath = path.resolve(process.cwd(), "components/app-shell.tsx");
  const source = fs.readFileSync(shellPath, "utf8");
  const sellerNav = extractSellerNavBlock(source);

  const requiredHrefs = [
    "/app/dashboard",
    "/app/actions",
    "/app/chat",
    "/app/account",
    "/app/integrations/ozon",
  ];
  for (const href of requiredHrefs) {
    assert.ok(sellerNav.includes(href), `seller nav should include ${href}`);
  }

  const requiredLabels = ["nav.dashboard", "nav.actions", "nav.aiChat", "nav.sellerAccount", "nav.ozonConnection"];
  for (const label of requiredLabels) {
    assert.ok(sellerNav.includes(label), `seller nav should reference ${label}`);
  }

  const hiddenHrefs = [
    "/app/products",
    "/app/advertising",
    "/app/critical-skus",
    "/app/stocks-replenishment",
    "/app/alerts",
    "/app/recommendations",
    "/app/prices",
    "/app/pricing",
    "/app/pricing-constraints",
  ];
  for (const href of hiddenHrefs) {
    assert.ok(!sellerNav.includes(href), `seller nav should not include ${href}`);
  }

  assert.ok(source.includes('href: "/app/admin"'), "admin nav item should remain in app shell");
  assert.ok(source.includes("showAdminNav"), "admin role-based nav logic should remain");

  const actionsPagePath = path.resolve(process.cwd(), "app/app/actions/page.tsx");
  assert.ok(fs.existsSync(actionsPagePath), "/app/actions page should exist");

  const legacyRoutes = [
    "app/app/products/page.tsx",
    "app/app/advertising/page.tsx",
    "app/app/critical-skus/page.tsx",
    "app/app/stocks-replenishment/page.tsx",
    "app/app/alerts/page.tsx",
    "app/app/recommendations/page.tsx",
  ];
  for (const route of legacyRoutes) {
    const routePath = path.resolve(process.cwd(), route);
    assert.ok(fs.existsSync(routePath), `legacy route file should remain: ${route}`);
  }

  console.log("app-shell-nav tests passed");
}

run();
