import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { listFilesRecursive } from "./atomic.mjs";
import { scanForSecrets } from "./sanitize.mjs";
import {
  analyzeHtmlDiscoverability,
  collectScreenshotRelPaths,
} from "./index-html.mjs";

const MIN_PNG_BYTES = 8_000;

export function contentChecksum(revisionRoot) {
  const h = crypto.createHash("sha256");
  for (const rel of listFilesRecursive(revisionRoot).sort()) {
    if (/\.(png|jpg|jpeg|webp|gif)$/i.test(rel)) continue;
    h.update(rel);
    h.update(fs.readFileSync(path.join(revisionRoot, rel)));
  }
  return h.digest("hex");
}

export function validateRevision(revisionRoot, { requirePreviousDiff = false } = {}) {
  const errors = [];
  const warnings = [];

  const required = [
    "index.html",
    "links.html",
    "manifest.json",
    "visible-content.md",
    "visible-content.txt",
    "metrics-reconciliation.md",
    "metrics-reconciliation.txt",
    "revision-diff.md",
    "revision-diff.txt",
    "content/dashboard.txt",
    "content/actions.txt",
    "data/dashboard.sanitized.json",
    "data/actions.sanitized.json",
    "data/action-details.sanitized.json",
    "data/publication-manifest.sanitized.json",
    "data/chat-context.sanitized.json",
    "diagnostics/capture-summary.md",
    "diagnostics/capture-summary.txt",
    "diagnostics/secret-scan-summary.md",
    "diagnostics/secret-scan-summary.txt",
    "screenshots/desktop/dashboard-full.png",
    "screenshots/desktop/actions-list.png",
    "screenshots/mobile/dashboard-full.png",
  ];

  for (const rel of required) {
    if (!fs.existsSync(path.join(revisionRoot, rel))) errors.push(`missing:${rel}`);
  }

  let manifest = null;
  try {
    manifest = JSON.parse(fs.readFileSync(path.join(revisionRoot, "manifest.json"), "utf8"));
  } catch (e) {
    errors.push(`manifest_invalid:${e}`);
  }

  if (manifest) {
    for (const k of [
      "bundle_id",
      "bundle_revision",
      "created_at",
      "refreshed_at",
      "expires_at",
      "seller_id",
      "publication_id",
      "capture_status",
    ]) {
      if (manifest[k] == null || manifest[k] === "") errors.push(`manifest_missing_field:${k}`);
    }
  }

  const shots = listFilesRecursive(path.join(revisionRoot, "screenshots"));
  if (shots.length < 10) errors.push(`screenshot_count_low:${shots.length}`);
  for (const rel of shots) {
    const full = path.join(revisionRoot, "screenshots", rel);
    const st = fs.statSync(full);
    if (st.size < MIN_PNG_BYTES) errors.push(`png_too_small:${rel}:${st.size}`);
    const buf = fs.readFileSync(full);
    if (buf[0] !== 0x89 || buf[1] !== 0x50) errors.push(`png_bad_magic:${rel}`);
  }

  const dashTxt = safeRead(path.join(revisionRoot, "content", "dashboard.txt"));
  for (const needle of [/пульс|Seller Pulse/i, /воронк/i, /остат/i]) {
    if (!needle.test(dashTxt)) errors.push(`dashboard_missing:${needle}`);
  }
  const actionsTxt = safeRead(path.join(revisionRoot, "content", "actions.txt"));
  if (actionsTxt.trim().length < 40) errors.push("actions_text_too_short");

  const secretHits = [];
  for (const rel of listFilesRecursive(revisionRoot)) {
    if (/\.(png|jpg|jpeg|webp|gif)$/i.test(rel)) continue;
    const text = fs.readFileSync(path.join(revisionRoot, rel), "utf8");
    for (const h of scanForSecrets(text)) secretHits.push({ file: rel, ...h });
  }
  if (secretHits.length) errors.push(`secrets_found:${secretHits.length}`);

  if (requirePreviousDiff) {
    const diff = safeRead(path.join(revisionRoot, "revision-diff.md"));
    if (!/previous_revision/i.test(diff)) errors.push("revision_diff_missing_previous");
  }

  // HTML discoverability for external web readers (static anchors, no JS).
  const screenshotRelPaths = collectScreenshotRelPaths(revisionRoot);
  const indexHtml = safeRead(path.join(revisionRoot, "index.html"));
  const linksHtml = safeRead(path.join(revisionRoot, "links.html"));
  const indexDisc = analyzeHtmlDiscoverability(indexHtml, { screenshotRelPaths });
  const linksDisc = analyzeHtmlDiscoverability(linksHtml, { screenshotRelPaths });

  if (indexDisc.has_script || linksDisc.has_script) errors.push("html_contains_script");
  const minShot = screenshotRelPaths.length;
  if (minShot > 0) {
    if (indexDisc.img_src_png_count < minShot) {
      errors.push(`index_img_src_png_low:${indexDisc.img_src_png_count}<${minShot}`);
    }
    // Each screenshot has an image-wrap <a> plus an "Open …" caption <a>.
    if (indexDisc.a_href_png_count < minShot) {
      errors.push(`index_a_href_png_low:${indexDisc.a_href_png_count}<${minShot}`);
    }
    if (minShot >= 30 && indexDisc.a_href_png_count < 30) {
      errors.push(`index_a_href_png_below_30:${indexDisc.a_href_png_count}`);
    }
    if (minShot >= 30 && indexDisc.img_src_png_count < 30) {
      errors.push(`index_img_src_png_below_30:${indexDisc.img_src_png_count}`);
    }
    if (indexDisc.missing_screenshot_anchors.length) {
      errors.push(`index_missing_anchors:${indexDisc.missing_screenshot_anchors.length}`);
    }
    if (linksDisc.a_href_png_count < minShot) {
      errors.push(`links_a_href_png_low:${linksDisc.a_href_png_count}<${minShot}`);
    }
  }
  if (!/href=["']links\.html/i.test(indexHtml)) errors.push("index_missing_links_page_anchor");
  if (!/href=["']visible-content\.txt/i.test(indexHtml)) {
    errors.push("index_missing_visible_content_txt");
  }

  return {
    ok: errors.length === 0,
    errors,
    warnings,
    screenshot_count: shots.length,
    content_checksum: contentChecksum(revisionRoot),
    secret_hits: secretHits,
    discoverability: {
      index: indexDisc,
      links: linksDisc,
    },
  };
}

function safeRead(p) {
  try {
    return fs.readFileSync(p, "utf8");
  } catch {
    return "";
  }
}
