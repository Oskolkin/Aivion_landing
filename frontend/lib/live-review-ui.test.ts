import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import type { AuthResponse } from "./auth-types";

function deriveShellFlags(auth: AuthResponse) {
  const isLiveReview = Boolean(auth.is_live_review);
  const isClassicReviewMode =
    Boolean(auth.is_review_mode || auth.read_only) && !isLiveReview;
  const visualDemoSeller = Boolean(auth.is_demo_seller) && !isLiveReview;
  const connectionDemo =
    Boolean(auth.is_demo_seller) || isClassicReviewMode || isLiveReview;
  return { isLiveReview, isClassicReviewMode, visualDemoSeller, connectionDemo };
}

function expectedNav(flags: ReturnType<typeof deriveShellFlags>) {
  const items = [
    "/app/dashboard",
    "/app/actions",
    "/app/chat",
    "/app/account",
    "/app/integrations/ozon",
  ];
  return items.filter((href) => {
    if ((flags.visualDemoSeller || flags.isClassicReviewMode) && href === "/app/integrations/ozon") {
      return false;
    }
    if (flags.isClassicReviewMode && href === "/app/account") {
      return false;
    }
    return true;
  });
}

function run() {
  const live: AuthResponse = {
    user: { id: 1, email: "demo@example.com", status: "active" },
    seller_account: { id: 1, name: "Demo", status: "active" },
    is_admin: false,
    is_demo_seller: true,
    is_review_mode: true,
    is_live_review: true,
    read_only: true,
  };
  const liveFlags = deriveShellFlags(live);
  assert.equal(liveFlags.isLiveReview, true);
  assert.equal(liveFlags.isClassicReviewMode, false);
  assert.equal(liveFlags.visualDemoSeller, false);
  assert.equal(liveFlags.connectionDemo, true);
  const liveNav = expectedNav(liveFlags);
  assert.deepEqual(liveNav, [
    "/app/dashboard",
    "/app/actions",
    "/app/chat",
    "/app/account",
    "/app/integrations/ozon",
  ]);

  const classic: AuthResponse = {
    user: { id: 1, email: "demo@example.com", status: "active" },
    seller_account: { id: 1, name: "Demo", status: "active" },
    is_admin: false,
    is_demo_seller: true,
    is_review_mode: true,
    read_only: true,
  };
  const classicFlags = deriveShellFlags(classic);
  assert.equal(classicFlags.isClassicReviewMode, true);
  assert.equal(classicFlags.isLiveReview, false);
  const classicNav = expectedNav(classicFlags);
  assert.ok(!classicNav.includes("/app/account"));
  assert.ok(!classicNav.includes("/app/integrations/ozon"));

  const demo: AuthResponse = {
    user: { id: 1, email: "demo@example.com", status: "active" },
    seller_account: { id: 1, name: "Demo", status: "active" },
    is_admin: false,
    is_demo_seller: true,
  };
  const demoFlags = deriveShellFlags(demo);
  assert.equal(demoFlags.visualDemoSeller, true);
  assert.equal(demoFlags.isClassicReviewMode, false);
  assert.ok(!expectedNav(demoFlags).includes("/app/integrations/ozon"));
  assert.ok(expectedNav(demoFlags).includes("/app/account"));

  const normal: AuthResponse = {
    user: { id: 2, email: "seller@example.com", status: "active" },
    seller_account: { id: 3, name: "Seller", status: "active" },
    is_admin: false,
  };
  const normalFlags = deriveShellFlags(normal);
  assert.deepEqual(expectedNav(normalFlags), [
    "/app/dashboard",
    "/app/actions",
    "/app/chat",
    "/app/account",
    "/app/integrations/ozon",
  ]);

  // AuthResponse type supports optional is_live_review
  const legacy: AuthResponse = {
    user: { id: 1, email: "x@y.z", status: "active" },
    seller_account: null,
    is_admin: false,
  };
  assert.equal(legacy.is_live_review, undefined);

  const layoutPath = path.resolve(process.cwd(), "app/app/layout.tsx");
  const layout = fs.readFileSync(layoutPath, "utf8");
  assert.ok(layout.includes("is_live_review"), "layout must branch on live review");
  assert.ok(layout.includes("isClassicReviewMode"), "layout must keep classic review mode");
  assert.ok(!layout.includes("review banner"), "layout should not hardcode banner text");

  const shellPath = path.resolve(process.cwd(), "components/app-shell.tsx");
  const shell = fs.readFileSync(shellPath, "utf8");
  assert.ok(shell.includes("review-mode-banner"), "classic review banner must remain");
  assert.ok(shell.includes('href: "/app/integrations/ozon"'), "ozon nav must remain in seller nav");

  const indexPath = path.resolve(process.cwd(), "app/live-review-index/page.tsx");
  assert.ok(fs.existsSync(indexPath), "live-review-index page must exist");
  const indexSrc = fs.readFileSync(indexPath, "utf8");
  assert.ok(indexSrc.includes("is_live_review"), "index gated on live review");
  for (const href of [
    "/app/dashboard",
    "/app/actions",
    "/app/chat",
    "/app/account",
    "/app/integrations/ozon",
  ]) {
    assert.ok(indexSrc.includes(href), `index must link ${href}`);
  }
  assert.ok(!indexSrc.includes("token"), "index must not mention tokens");
  assert.ok(!indexSrc.includes("secret"), "index must not mention secrets");
  assert.ok(!indexSrc.includes("/app/admin"), "index must not link admin");

  console.log("live-review-ui tests passed");
}

run();
