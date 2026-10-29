import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { sanitizeObject, sanitizeText, scanForSecrets } from "../src/sanitize.mjs";
import { atomicActivateRevision, sha256Dir } from "../src/atomic.mjs";
import { buildRevisionDiff } from "../src/revision-diff.mjs";

test("sanitizeObject redacts tokens and passwords", () => {
  const jwtish = ["eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "aaa", "bbb"].join(".");
  const out = sanitizeObject({
    title: "ok",
    password: "secret123",
    nested: { access_token: "abc", metric: 12 },
    note: `Bearer ${jwtish}`,
  });
  assert.equal(out.title, "ok");
  assert.equal(out.password, "[REDACTED]");
  assert.equal(out.nested.access_token, "[REDACTED]");
  assert.equal(out.nested.metric, 12);
  assert.match(out.note, /REDACTED/);
});

test("scanForSecrets finds sk- keys", () => {
  const fake = ["sk", "abcdefghijklmnopqrstuvwxyz123456"].join("-");
  const hits = scanForSecrets(`key=${fake}`);
  assert.ok(hits.length >= 1);
});

test("sanitizeText redacts bearer-like values", () => {
  const tokenish = ["Bearer", "aaaa.bbbb.cccc"].join(" ");
  const out = sanitizeText(`Authorization: ${tokenish}`);
  assert.match(out, /REDACTED/);
});

test("atomicActivateRevision switches without mixing", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "audit-atomic-"));
  const bundleRoot = path.join(root, "bundle");
  const published = path.join(root, "published");
  fs.mkdirSync(path.join(bundleRoot, "revisions"), { recursive: true });

  const stage1 = path.join(root, "stage1");
  fs.mkdirSync(stage1);
  fs.writeFileSync(path.join(stage1, "index.html"), "v1");
  atomicActivateRevision({
    bundleRoot,
    revisionName: "rev-001",
    revisionDir: stage1,
    publishedRoot: published,
    bundleId: "testid",
  });
  assert.equal(fs.readFileSync(path.join(published, "testid", "index.html"), "utf8"), "v1");

  const stage2 = path.join(root, "stage2");
  fs.mkdirSync(stage2);
  fs.writeFileSync(path.join(stage2, "index.html"), "v2");
  atomicActivateRevision({
    bundleRoot,
    revisionName: "rev-002",
    revisionDir: stage2,
    publishedRoot: published,
    bundleId: "testid",
  });
  assert.equal(fs.readFileSync(path.join(published, "testid", "index.html"), "utf8"), "v2");
  assert.equal(fs.readFileSync(path.join(bundleRoot, "revisions", "rev-001", "index.html"), "utf8"), "v1");
  fs.rmSync(root, { recursive: true, force: true });
});

test("buildRevisionDiff first revision", () => {
  const md = buildRevisionDiff({
    previousManifest: null,
    nextManifest: {
      bundle_revision: 1,
      publication_id: 2,
      source_run_id: "r1",
      metric_date: "2026-01-01",
      deployed_frontend_sha: "abc",
    },
    previousRoot: null,
    nextRoot: null,
  });
  assert.match(md, /previous_revision = none/);
});

test("sha256Dir stable", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "audit-hash-"));
  fs.writeFileSync(path.join(root, "a.txt"), "x");
  const h1 = sha256Dir(root);
  const h2 = sha256Dir(root);
  assert.equal(h1, h2);
  fs.rmSync(root, { recursive: true, force: true });
});

test("index and links html expose static screenshot anchors", async () => {
  const { buildIndexHtml, buildLinksHtml, analyzeHtmlDiscoverability, writePlainTextDocumentCopies } =
    await import("../src/index-html.mjs");
  const shots = [];
  for (let i = 1; i <= 30; i++) {
    shots.push(`screenshots/desktop/shot-${String(i).padStart(2, "0")}.png`);
  }
  const manifest = {
    bundle_revision: 3,
    created_at: "t",
    refreshed_at: "t",
    expires_at: "t",
    deployed_frontend_sha: "abc",
    deployed_backend_sha: "abc",
    deployed_worker_sha: "abc",
    seller_id: 1,
    publication_id: 2,
    source_run_id: "1",
    source_mode: "unified",
    metric_date: "2026-01-01",
    dashboard_block_count: 7,
    action_count: 10,
    action_detail_count: 8,
    screenshot_count: 30,
    content_checksum: "x",
    revision_checksum: "y",
    previous_revision: 2,
    capture_status: "PASS",
  };
  const index = buildIndexHtml(manifest, { screenshotRelPaths: shots });
  const links = buildLinksHtml(manifest, { screenshotRelPaths: shots });
  const disc = analyzeHtmlDiscoverability(index, { screenshotRelPaths: shots });
  assert.equal(disc.has_script, false);
  assert.ok(disc.a_href_png_count >= 30);
  assert.ok(disc.img_src_png_count >= 30);
  assert.equal(disc.missing_screenshot_anchors.length, 0);
  assert.match(index, /href="links\.html\?v=3"/);
  assert.match(index, /href="visible-content\.txt\?v=3"/);
  assert.match(index, /Open shot-01\.png/);
  const linksDisc = analyzeHtmlDiscoverability(links, { screenshotRelPaths: shots });
  assert.ok(linksDisc.a_href_png_count >= 30);
  assert.match(links, /visible-content\.txt/);

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "audit-txt-"));
  fs.writeFileSync(path.join(tmp, "visible-content.md"), "# hi");
  fs.writeFileSync(path.join(tmp, "metrics-reconciliation.md"), "m");
  fs.writeFileSync(path.join(tmp, "revision-diff.md"), "d");
  fs.mkdirSync(path.join(tmp, "diagnostics"));
  fs.writeFileSync(path.join(tmp, "diagnostics", "capture-summary.md"), "c");
  fs.writeFileSync(path.join(tmp, "diagnostics", "secret-scan-summary.md"), "s");
  const written = writePlainTextDocumentCopies(tmp);
  assert.ok(written.includes("visible-content.txt"));
  assert.equal(fs.readFileSync(path.join(tmp, "visible-content.txt"), "utf8"), "# hi");
  fs.rmSync(tmp, { recursive: true, force: true });
});
