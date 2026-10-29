import fs from "node:fs";
import path from "node:path";

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Human label for screenshot relative path. */
export function screenshotAlt(rel) {
  // screenshots/desktop/dashboard-full.png -> Dashboard — desktop — full page
  const parts = String(rel).replace(/\\/g, "/").split("/");
  const file = (parts[parts.length - 1] || "").replace(/\.png$/i, "");
  const viewport = parts[parts.length - 2] || "view";
  const title = file
    .split("-")
    .map((w) => (w.length ? w[0].toUpperCase() + w.slice(1) : w))
    .join(" ");
  return `${title} — ${viewport}`;
}

export function screenshotOpenLabel(rel) {
  const file = String(rel).replace(/\\/g, "/").split("/").pop() || rel;
  return `Open ${file}`;
}

function docLinks(v) {
  return [
    ["manifest.json", "manifest.json"],
    ["visible-content.txt", "visible-content.txt (preferred)"],
    ["visible-content.md", "visible-content.md"],
    ["metrics-reconciliation.txt", "metrics-reconciliation.txt (preferred)"],
    ["metrics-reconciliation.md", "metrics-reconciliation.md"],
    ["revision-diff.txt", "revision-diff.txt (preferred)"],
    ["revision-diff.md", "revision-diff.md"],
    ["data/dashboard.sanitized.json", "dashboard.sanitized.json"],
    ["data/actions.sanitized.json", "actions.sanitized.json"],
    ["data/action-details.sanitized.json", "action-details.sanitized.json"],
    ["data/publication-manifest.sanitized.json", "publication-manifest.sanitized.json"],
    ["data/chat-context.sanitized.json", "chat-context.sanitized.json"],
    ["diagnostics/capture-summary.txt", "capture-summary.txt"],
    ["diagnostics/secret-scan-summary.txt", "secret-scan-summary.txt"],
    ["links.html", "All direct audit files (links.html)"],
  ]
    .map(
      ([href, label]) =>
        `<li><a href="${esc(href)}?v=${esc(v)}">${esc(label)}</a></li>`,
    )
    .join("\n        ");
}

function screenshotFigures(screenshotRelPaths, v) {
  return screenshotRelPaths
    .map((rel) => {
      const href = `${esc(rel)}?v=${esc(v)}`;
      const alt = esc(screenshotAlt(rel));
      const open = esc(screenshotOpenLabel(rel));
      return `<figure class="shot">
  <a href="${href}"><img src="${href}" alt="${alt}" loading="lazy" width="480"></a>
  <figcaption><a href="${href}">${open}</a></figcaption>
</figure>`;
    })
    .join("\n");
}

/**
 * Build a no-JS index.html for the audit bundle.
 * All screenshot anchors/imgs are static (no JS generation).
 */
export function buildIndexHtml(manifest, { screenshotRelPaths = [] } = {}) {
  const v = manifest.bundle_revision;
  const thumbs = screenshotFigures(screenshotRelPaths, v);

  return `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex, nofollow, noarchive, nosnippet">
  <meta name="referrer" content="no-referrer">
  <title>Aivion External Audit</title>
  <style>
    :root { color-scheme: light; --bg:#f6f8fb; --card:#fff; --text:#0f172a; --muted:#64748b; --line:#e2e8f0; }
    * { box-sizing: border-box; }
    body { margin:0; font-family: Georgia, "Times New Roman", serif; background:var(--bg); color:var(--text); line-height:1.45; }
    header { padding:2rem 1.25rem 1rem; border-bottom:1px solid var(--line); background:linear-gradient(180deg,#eef4ff,var(--bg)); }
    h1 { margin:0 0 .35rem; font-size:1.75rem; }
    .badge { display:inline-block; font-size:.8rem; color:#075985; background:#e0f2fe; padding:.2rem .55rem; border-radius:4px; }
    .hero-link { display:inline-block; margin-top:.75rem; font-size:1.05rem; font-weight:700; }
    main { max-width:1100px; margin:0 auto; padding:1.25rem; }
    section { background:var(--card); border:1px solid var(--line); border-radius:10px; padding:1rem 1.1rem; margin:1rem 0; }
    h2 { margin:0 0 .75rem; font-size:1.15rem; }
    dl { display:grid; grid-template-columns: 12rem 1fr; gap:.35rem .75rem; margin:0; }
    dt { color:var(--muted); } dd { margin:0; font-family: ui-monospace, Consolas, monospace; font-size:.92rem; }
    ul { margin:.25rem 0 0 1.1rem; }
    a { color:#0369a1; }
    .shots { display:flex; flex-wrap:wrap; gap:16px; }
    .shot { margin:0; width:480px; max-width:100%; font-size:.85rem; }
    .shot img { width:100%; height:auto; border:1px solid var(--line); border-radius:6px; background:#fff; display:block; }
    .shot figcaption { margin-top:.35rem; }
    footer { padding:1.5rem 1.25rem 2.5rem; color:var(--muted); font-size:.9rem; text-align:center; }
  </style>
</head>
<body>
  <header>
    <h1>Aivion External Audit</h1>
    <p class="badge">Read-only static snapshot — not a live application session</p>
    <p><a class="hero-link" href="links.html?v=${esc(v)}">All direct audit files</a></p>
  </header>
  <main>
    <section>
      <h2>Snapshot metadata</h2>
      <dl>
        <dt>Created</dt><dd>${esc(manifest.created_at)}</dd>
        <dt>Refreshed</dt><dd>${esc(manifest.refreshed_at)}</dd>
        <dt>Expires</dt><dd>${esc(manifest.expires_at)}</dd>
        <dt>Revision</dt><dd>${esc(manifest.bundle_revision)}</dd>
        <dt>Frontend SHA</dt><dd>${esc(manifest.deployed_frontend_sha)}</dd>
        <dt>Backend SHA</dt><dd>${esc(manifest.deployed_backend_sha)}</dd>
        <dt>Worker SHA</dt><dd>${esc(manifest.deployed_worker_sha)}</dd>
        <dt>Seller</dt><dd>${esc(manifest.seller_id)}</dd>
        <dt>Publication</dt><dd>${esc(manifest.publication_id)}</dd>
        <dt>Source run</dt><dd>${esc(manifest.source_run_id)}</dd>
        <dt>Source mode</dt><dd>${esc(manifest.source_mode)}</dd>
        <dt>Metric date</dt><dd>${esc(manifest.metric_date)}</dd>
        <dt>Dashboard blocks</dt><dd>${esc(manifest.dashboard_block_count)}</dd>
        <dt>Actions</dt><dd>${esc(manifest.action_count)}</dd>
        <dt>Action details</dt><dd>${esc(manifest.action_detail_count)}</dd>
        <dt>Screenshots</dt><dd>${esc(manifest.screenshot_count)}</dd>
        <dt>Content checksum</dt><dd>${esc(manifest.content_checksum)}</dd>
        <dt>Revision checksum</dt><dd>${esc(manifest.revision_checksum)}</dd>
        <dt>Previous revision</dt><dd>${esc(manifest.previous_revision ?? "none")}</dd>
        <dt>Capture status</dt><dd>${esc(manifest.capture_status)}</dd>
      </dl>
    </section>
    <section>
      <h2>Documents</h2>
      <p>Prefer <code>.txt</code> documents for external readers that do not support Markdown MIME types.</p>
      <ul>
        ${docLinks(v)}
      </ul>
    </section>
    <section>
      <h2>Screenshots</h2>
      <p>Each image is a static HTML link. Open the full PNG via the caption link.</p>
      <div class="shots">
${thumbs || "<p>No screenshots listed.</p>"}
      </div>
    </section>
  </main>
  <footer>Static audit bundle. No live API access. Bundle removed after expires_at.</footer>
</body>
</html>
`;
}

/**
 * Flat link directory for crawlers / web readers (no JS).
 */
export function buildLinksHtml(manifest, { screenshotRelPaths = [], extraDocs = [] } = {}) {
  const v = manifest.bundle_revision;
  const docs = [
    ["manifest.json", "Manifest JSON"],
    ["visible-content.txt", "Visible content (text/plain)"],
    ["metrics-reconciliation.txt", "Metrics reconciliation (text/plain)"],
    ["revision-diff.txt", "Revision diff (text/plain)"],
    ["visible-content.md", "Visible content (markdown)"],
    ["metrics-reconciliation.md", "Metrics reconciliation (markdown)"],
    ["revision-diff.md", "Revision diff (markdown)"],
    ["data/dashboard.sanitized.json", "Dashboard sanitized JSON"],
    ["data/actions.sanitized.json", "Actions sanitized JSON"],
    ["data/action-details.sanitized.json", "Action details sanitized JSON"],
    ["data/publication-manifest.sanitized.json", "Publication manifest sanitized JSON"],
    ["data/chat-context.sanitized.json", "Chat context sanitized JSON"],
    ["diagnostics/capture-summary.txt", "Capture summary (text/plain)"],
    ["diagnostics/secret-scan-summary.txt", "Secret scan summary (text/plain)"],
    ["diagnostics/capture-summary.md", "Capture summary (markdown)"],
    ["diagnostics/secret-scan-summary.md", "Secret scan summary (markdown)"],
    ["index.html", "Audit index"],
    ...extraDocs,
  ];

  const docItems = docs
    .map(
      ([href, label]) =>
        `  <li><a href="${esc(href)}?v=${esc(v)}">${esc(label)}</a></li>`,
    )
    .join("\n");

  const shotItems = screenshotRelPaths
    .map((rel) => {
      const label = screenshotAlt(rel);
      return `  <li><a href="${esc(rel)}?v=${esc(v)}">${esc(label)}</a></li>`;
    })
    .join("\n");

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex, nofollow, noarchive, nosnippet">
  <meta name="referrer" content="no-referrer">
  <title>Aivion External Audit — All direct files</title>
  <style>
    body { font-family: Georgia, "Times New Roman", serif; margin: 1.5rem; line-height: 1.5; color: #0f172a; }
    h1 { font-size: 1.4rem; }
    h2 { font-size: 1.1rem; margin-top: 1.5rem; }
    a { color: #0369a1; }
  </style>
</head>
<body>
  <h1>Aivion External Audit — All direct files</h1>
  <p>Revision ${esc(v)}. Static links only. No JavaScript.</p>
  <p><a href="index.html?v=${esc(v)}">Back to index</a></p>
  <h2>Documents</h2>
  <ul>
${docItems}
  </ul>
  <h2>Screenshots</h2>
  <ul>
${shotItems}
  </ul>
</body>
</html>
`;
}

export function collectScreenshotRelPaths(revisionRoot) {
  const out = [];
  const base = path.join(revisionRoot, "screenshots");
  for (const vp of ["desktop", "laptop", "mobile"]) {
    const dir = path.join(base, vp);
    if (!fs.existsSync(dir)) continue;
    for (const f of fs.readdirSync(dir).filter((x) => x.endsWith(".png")).sort()) {
      out.push(`screenshots/${vp}/${f}`);
    }
  }
  return out;
}

/** Copy .md → .txt (and listed diagnostics) for text/plain readers. */
export function writePlainTextDocumentCopies(revisionRoot) {
  const pairs = [
    ["visible-content.md", "visible-content.txt"],
    ["metrics-reconciliation.md", "metrics-reconciliation.txt"],
    ["revision-diff.md", "revision-diff.txt"],
    ["diagnostics/capture-summary.md", "diagnostics/capture-summary.txt"],
    ["diagnostics/secret-scan-summary.md", "diagnostics/secret-scan-summary.txt"],
  ];
  const written = [];
  for (const [src, dest] of pairs) {
    const from = path.join(revisionRoot, src);
    const to = path.join(revisionRoot, dest);
    if (!fs.existsSync(from)) continue;
    fs.mkdirSync(path.dirname(to), { recursive: true });
    fs.copyFileSync(from, to);
    written.push(dest);
  }
  return written;
}

/**
 * Count screenshot anchors/images in static HTML source.
 */
export function analyzeHtmlDiscoverability(html, { screenshotRelPaths = [] } = {}) {
  const aHrefPng = [...html.matchAll(/<a\s+[^>]*href=["']([^"']+\.png(?:\?[^"']*)?)["']/gi)].map(
    (m) => m[1],
  );
  const imgSrcPng = [...html.matchAll(/<img\s+[^>]*src=["']([^"']+\.png(?:\?[^"']*)?)["']/gi)].map(
    (m) => m[1],
  );
  const scriptTags = /<script[\s>]/i.test(html);
  const missingAnchors = screenshotRelPaths.filter(
    (rel) => !aHrefPng.some((h) => h.includes(rel) || decodeURIComponent(h).includes(rel)),
  );
  return {
    a_href_png_count: aHrefPng.length,
    img_src_png_count: imgSrcPng.length,
    has_script: scriptTags,
    missing_screenshot_anchors: missingAnchors,
  };
}
