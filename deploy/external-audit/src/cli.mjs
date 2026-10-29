#!/usr/bin/env node
/**
 * CLI for Aivion external-audit capture / validate / assemble.
 * Password is read only from --password-file. Never print secrets.
 */

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { captureBundle, DEFAULT_BASE_URL, DEFAULT_DEMO_EMAIL } from "./capture.mjs";
import { validateRevision, contentChecksum } from "./validate.mjs";
import {
  buildIndexHtml,
  buildLinksHtml,
  collectScreenshotRelPaths,
  writePlainTextDocumentCopies,
  analyzeHtmlDiscoverability,
} from "./index-html.mjs";
import { buildRevisionDiff } from "./revision-diff.mjs";
import { scanForSecrets } from "./sanitize.mjs";
import {
  sha256Dir,
  listFilesRecursive,
  writeJson,
  readJson,
  atomicActivateRevision,
} from "./atomic.mjs";

export function parseArgs(argv) {
  const args = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith("--")) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (!next || next.startsWith("--")) {
        args[key] = true;
      } else {
        args[key] = next;
        i++;
      }
    } else {
      args._.push(a);
    }
  }
  return args;
}

export function readPasswordFile(filePath) {
  if (!filePath) throw new Error("--password-file is required");
  const raw = fs.readFileSync(filePath, "utf8");
  const password = raw.replace(/^\uFEFF/, "").replace(/\r?\n$/, "");
  if (!password) throw new Error("password file is empty");
  return password;
}

export async function cmdCapture(args) {
  const outDir = args.out || args["out-dir"];
  if (!outDir) throw new Error("capture: --out <dir> is required");
  const password = readPasswordFile(args["password-file"]);
  const meta = await captureBundle({
    outDir: path.resolve(outDir),
    password,
    baseUrl: args["base-url"] || process.env.BASE_URL || DEFAULT_BASE_URL,
    email: args.email || process.env.DEMO_EMAIL || DEFAULT_DEMO_EMAIL,
    sellerId: args["seller-id"] || 1,
  });
  // Do not print secrets; metadata only.
  process.stdout.write(JSON.stringify(meta) + "\n");
  return meta;
}

export function cmdValidate(args) {
  const revision = args.revision;
  if (!revision) throw new Error("validate: --revision <dir> is required");
  const result = validateRevision(path.resolve(revision), {
    requirePreviousDiff: Boolean(args["require-previous-diff"]),
  });
  process.stdout.write(
    JSON.stringify({
      ok: result.ok,
      errors: result.errors,
      warnings: result.warnings,
      screenshot_count: result.screenshot_count,
      content_checksum: result.content_checksum,
    }) + "\n",
  );
  if (!result.ok) process.exitCode = 1;
  return result;
}

/**
 * Assemble a captured staging dir into a publishable revision:
 * manifest, index.html, revision-diff, secret scan, validate.
 */
export function cmdAssemble(args) {
  const work = args.work;
  const bundleId = args["bundle-id"];
  const revision = Number(args.revision);
  const metaInPath = args["meta-in"];
  if (!work) throw new Error("assemble: --work <staging> is required");
  if (!bundleId) throw new Error("assemble: --bundle-id <id> is required");
  if (!Number.isFinite(revision) || revision < 1) {
    throw new Error("assemble: --revision <n> is required (positive integer)");
  }
  if (!metaInPath) throw new Error("assemble: --meta-in <json> is required");

  const revisionRoot = path.resolve(work);
  const meta = readJson(path.resolve(metaInPath));

  const createdAt = args["created-at"] || new Date().toISOString();
  const refreshedAt = args["refreshed-at"] || createdAt;
  const expiresAt = args["expires-at"];
  if (!expiresAt) throw new Error("assemble: --expires-at <iso> is required");

  const deployedSha = args["deployed-sha"] || args["deployed-frontend-sha"] || "";
  const deployedFrontendSha = args["deployed-frontend-sha"] || deployedSha;
  const deployedBackendSha = args["deployed-backend-sha"] || deployedSha;
  const deployedWorkerSha = args["deployed-worker-sha"] || deployedSha;

  let previousManifest = null;
  let previousRoot = null;
  if (args["previous-manifest"]) {
    previousManifest = readJson(path.resolve(args["previous-manifest"]));
  }
  if (args["previous-revision-dir"]) {
    previousRoot = path.resolve(args["previous-revision-dir"]);
    if (!previousManifest) {
      const prevManifestPath = path.join(previousRoot, "manifest.json");
      if (fs.existsSync(prevManifestPath)) previousManifest = readJson(prevManifestPath);
    }
  }

  const screenshotRelPaths = collectScreenshotRelPaths(revisionRoot);
  const screenshotCount =
    meta.screenshot_count != null ? meta.screenshot_count : screenshotRelPaths.length;

  // Provisional manifest (checksums filled after artifacts land).
  const manifest = {
    bundle_id: bundleId,
    bundle_revision: revision,
    created_at: createdAt,
    refreshed_at: refreshedAt,
    expires_at: expiresAt,
    deployed_frontend_sha: deployedFrontendSha,
    deployed_backend_sha: deployedBackendSha,
    deployed_worker_sha: deployedWorkerSha,
    seller_id: meta.seller_id ?? 1,
    publication_id: meta.publication_id ?? null,
    source_run_id: meta.source_run_id ?? null,
    source_mode: meta.source_mode ?? null,
    metric_date: meta.metric_date ?? null,
    dashboard_block_count: meta.dashboard_block_count ?? 0,
    action_count: meta.action_count ?? 0,
    action_detail_count: meta.action_detail_count ?? 0,
    screenshot_count: screenshotCount,
    content_checksum: "",
    revision_checksum: "",
    previous_revision: previousManifest?.bundle_revision ?? null,
    capture_status: "PASS",
    chat_context_present: Boolean(meta.chat_context_present),
  };

  // Secret scan before finalize
  const secretHits = [];
  for (const rel of listFilesRecursive(revisionRoot)) {
    if (/\.(png|jpe?g|webp|gif)$/i.test(rel)) continue;
    const text = fs.readFileSync(path.join(revisionRoot, rel), "utf8");
    for (const h of scanForSecrets(text)) secretHits.push({ file: rel, ...h });
  }
  const secretSummary = [
    "# Secret scan summary",
    "",
    `- scanned_at: ${new Date().toISOString()}`,
    `- hits: ${secretHits.length}`,
    "",
    ...(secretHits.length
      ? secretHits.map((h) => `- ${h.file}: ${h.sample || "hit"}`)
      : ["- none"]),
    "",
  ].join("\n");
  fs.mkdirSync(path.join(revisionRoot, "diagnostics"), { recursive: true });
  fs.writeFileSync(path.join(revisionRoot, "diagnostics", "secret-scan-summary.md"), secretSummary, "utf8");

  if (secretHits.length) {
    manifest.capture_status = "FAIL";
    writeJson(path.join(revisionRoot, "manifest.json"), manifest);
    process.stderr.write(`secret scan failed: ${secretHits.length} hit(s)\n`);
    process.exitCode = 1;
    printAssembleSummary(manifest, { omitBundleId: process.env.AUDIT_OMIT_BUNDLE_ID === "1" });
    return { ok: false, manifest, secretHits };
  }

  const diffMd = buildRevisionDiff({
    previousManifest,
    nextManifest: manifest,
    previousRoot,
    nextRoot: revisionRoot,
  });
  fs.writeFileSync(path.join(revisionRoot, "revision-diff.md"), diffMd, "utf8");

  // Prefer text/plain copies for external web readers.
  writePlainTextDocumentCopies(revisionRoot);

  const writeHtmlBundle = () => {
    fs.writeFileSync(
      path.join(revisionRoot, "index.html"),
      buildIndexHtml(manifest, { screenshotRelPaths }),
      "utf8",
    );
    fs.writeFileSync(
      path.join(revisionRoot, "links.html"),
      buildLinksHtml(manifest, { screenshotRelPaths }),
      "utf8",
    );
  };
  writeHtmlBundle();

  // Fill checksums after text artifacts exist (exclude images for content_checksum).
  manifest.content_checksum = contentChecksum(revisionRoot);
  manifest.revision_checksum = "pending";
  writeJson(path.join(revisionRoot, "manifest.json"), manifest);
  manifest.revision_checksum = sha256Dir(revisionRoot);
  writeJson(path.join(revisionRoot, "manifest.json"), manifest);

  // Rebuild HTML with final checksums / revision query params.
  writeHtmlBundle();
  // Re-copy txt after secret-scan / diff already written (idempotent).
  writePlainTextDocumentCopies(revisionRoot);

  const validation = validateRevision(revisionRoot, {
    requirePreviousDiff: Boolean(previousManifest),
  });
  if (!validation.ok) {
    manifest.capture_status = "FAIL";
    writeJson(path.join(revisionRoot, "manifest.json"), manifest);
    process.stderr.write(`validate failed:\n${validation.errors.join("\n")}\n`);
    process.exitCode = 1;
    printAssembleSummary(manifest, { omitBundleId: process.env.AUDIT_OMIT_BUNDLE_ID === "1" });
    return { ok: false, manifest, validation };
  }

  manifest.capture_status = "PASS";
  // Optional relative public paths for consumers (no absolute audit URL).
  manifest.public_relative_paths = {
    index: "index.html",
    links: "links.html",
    screenshots: screenshotRelPaths,
    documents: [
      "visible-content.txt",
      "metrics-reconciliation.txt",
      "revision-diff.txt",
      "manifest.json",
    ],
  };
  manifest.content_checksum = contentChecksum(revisionRoot);
  writeJson(path.join(revisionRoot, "manifest.json"), manifest);
  manifest.revision_checksum = sha256Dir(revisionRoot);
  writeJson(path.join(revisionRoot, "manifest.json"), manifest);
  writeHtmlBundle();

  const disc = analyzeHtmlDiscoverability(
    fs.readFileSync(path.join(revisionRoot, "index.html"), "utf8"),
    { screenshotRelPaths },
  );
  if (disc.a_href_png_count < screenshotRelPaths.length || disc.img_src_png_count < screenshotRelPaths.length) {
    process.stderr.write(
      `html discoverability weak: a=${disc.a_href_png_count} img=${disc.img_src_png_count} shots=${screenshotRelPaths.length}\n`,
    );
  }

  printAssembleSummary(manifest, { omitBundleId: process.env.AUDIT_OMIT_BUNDLE_ID === "1" });
  return { ok: true, manifest, validation, discoverability: disc };
}

export function printAssembleSummary(manifest, { omitBundleId = false } = {}) {
  const summary = {
    revision: manifest.bundle_revision,
    publication_id: manifest.publication_id,
    screenshot_count: manifest.screenshot_count,
    capture_status: manifest.capture_status,
    content_checksum: manifest.content_checksum,
  };
  if (!omitBundleId) summary.bundle_id = manifest.bundle_id;
  process.stdout.write(JSON.stringify(summary) + "\n");
}

export function cmdActivate(args) {
  const bundleRoot = args["bundle-root"];
  const revisionName = args["revision-name"];
  const revisionDir = args["revision-dir"];
  const publishedRoot = args["published-root"];
  const bundleId = args["bundle-id"];
  if (!bundleRoot || !revisionName || !revisionDir || !publishedRoot || !bundleId) {
    throw new Error(
      "activate: --bundle-root --revision-name --revision-dir --published-root --bundle-id required",
    );
  }
  const result = atomicActivateRevision({
    bundleRoot: path.resolve(bundleRoot),
    revisionName,
    revisionDir: path.resolve(revisionDir),
    publishedRoot: path.resolve(publishedRoot),
    bundleId,
  });
  process.stdout.write(JSON.stringify({ ok: true, revision: result.revisionName }) + "\n");
  return result;
}

export function usage() {
  return `Usage:
  node src/cli.mjs capture --out <dir> --base-url <url> --email <email> --password-file <path> [--seller-id 1]
  node src/cli.mjs validate --revision <dir>
  node src/cli.mjs assemble --work <staging> --bundle-id <id> --revision <n> --meta-in <json> [--previous-revision-dir <dir>] [--previous-manifest <json>] --deployed-sha <sha> --expires-at <iso> --created-at <iso>
  node src/cli.mjs activate --bundle-root <dir> --revision-name rev-NNN --revision-dir <dir> --published-root <dir> --bundle-id <id>
`;
}

async function main() {
  const argv = process.argv.slice(2);
  const cmd = argv[0];
  const args = parseArgs(argv.slice(1));

  if (!cmd || cmd === "help" || cmd === "-h" || cmd === "--help") {
    process.stdout.write(usage());
    return;
  }

  try {
    if (cmd === "capture") {
      await cmdCapture(args);
    } else if (cmd === "validate") {
      cmdValidate(args);
    } else if (cmd === "assemble") {
      cmdAssemble(args);
    } else if (cmd === "activate") {
      cmdActivate(args);
    } else {
      process.stderr.write(`unknown command: ${cmd}\n${usage()}`);
      process.exitCode = 2;
    }
  } catch (err) {
    // Never echo password or token-bearing messages beyond Error.message without secrets.
    process.stderr.write(String(err?.message || err) + "\n");
    process.exitCode = 1;
  }
}

const isDirect =
  process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url));

if (isDirect) {
  await main();
}
