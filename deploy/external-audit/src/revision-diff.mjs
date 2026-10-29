import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { listFilesRecursive } from "./atomic.mjs";

function fileHash(root, rel) {
  const p = path.join(root, rel);
  if (!fs.existsSync(p)) return null;
  return crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex");
}

export function buildRevisionDiff({ previousManifest, nextManifest, previousRoot, nextRoot }) {
  if (!previousManifest || !previousRoot) {
    return `# Revision diff

previous_revision = none

This is the first published revision of the bundle.

- bundle_revision: ${nextManifest.bundle_revision}
- publication_id: ${nextManifest.publication_id}
- source_run_id: ${nextManifest.source_run_id}
- metric_date: ${nextManifest.metric_date}
- deployed_frontend_sha: ${nextManifest.deployed_frontend_sha}
`;
  }

  const lines = [
    "# Revision diff",
    "",
    `previous_revision: ${previousManifest.bundle_revision}`,
    `next_revision: ${nextManifest.bundle_revision}`,
    "",
    "## Identifiers",
    "",
    `| Field | Previous | Next |`,
    `| --- | --- | --- |`,
    `| publication_id | ${previousManifest.publication_id} | ${nextManifest.publication_id} |`,
    `| source_run_id | ${previousManifest.source_run_id} | ${nextManifest.source_run_id} |`,
    `| metric_date | ${previousManifest.metric_date} | ${nextManifest.metric_date} |`,
    `| deployed_frontend_sha | ${previousManifest.deployed_frontend_sha} | ${nextManifest.deployed_frontend_sha} |`,
    `| deployed_backend_sha | ${previousManifest.deployed_backend_sha} | ${nextManifest.deployed_backend_sha} |`,
    `| dashboard_block_count | ${previousManifest.dashboard_block_count} | ${nextManifest.dashboard_block_count} |`,
    `| action_count | ${previousManifest.action_count} | ${nextManifest.action_count} |`,
    `| content_checksum | ${previousManifest.content_checksum} | ${nextManifest.content_checksum} |`,
    `| revision_checksum | ${previousManifest.revision_checksum} | ${nextManifest.revision_checksum} |`,
    "",
  ];

  // Action titles
  const prevActions = readActionTitles(previousRoot);
  const nextActions = readActionTitles(nextRoot);
  const added = nextActions.filter((t) => !prevActions.includes(t));
  const removed = prevActions.filter((t) => !nextActions.includes(t));
  lines.push("## Actions titles", "");
  lines.push(`- previous count: ${prevActions.length}`);
  lines.push(`- next count: ${nextActions.length}`);
  if (added.length) lines.push(`- appeared: ${added.map((t) => JSON.stringify(t)).join(", ")}`);
  if (removed.length) lines.push(`- disappeared: ${removed.map((t) => JSON.stringify(t)).join(", ")}`);
  lines.push("");

  // Screenshot hash changes
  lines.push("## Screenshot hashes", "");
  const shots = new Set([
    ...listFilesRecursive(path.join(previousRoot, "screenshots")).map((r) => `screenshots/${r}`),
    ...listFilesRecursive(path.join(nextRoot, "screenshots")).map((r) => `screenshots/${r}`),
  ]);
  let changed = 0;
  for (const rel of [...shots].sort()) {
    const a = fileHash(previousRoot, rel);
    const b = fileHash(nextRoot, rel);
    if (a !== b) {
      changed++;
      lines.push(`- changed: ${rel}`);
      lines.push(`  - previous: ${a ?? "missing"}`);
      lines.push(`  - next: ${b ?? "missing"}`);
    }
  }
  lines.push(`- changed_screenshot_count: ${changed}`);
  lines.push("");

  // Text block hashes
  for (const rel of ["content/dashboard.txt", "content/actions.txt", "visible-content.md"]) {
    const a = fileHash(previousRoot, rel);
    const b = fileHash(nextRoot, rel);
    lines.push(`## Text: ${rel}`);
    lines.push(`- previous: ${a ?? "missing"}`);
    lines.push(`- next: ${b ?? "missing"}`);
    lines.push(`- changed: ${a !== b}`);
    lines.push("");
  }

  lines.push("_Diff records factual differences only; no correctness judgment._");
  lines.push("");
  return lines.join("\n");
}

function readActionTitles(root) {
  const p = path.join(root, "data", "actions.sanitized.json");
  if (!fs.existsSync(p)) return [];
  try {
    const data = JSON.parse(fs.readFileSync(p, "utf8"));
    const items = data.items || data.actions || data || [];
    if (!Array.isArray(items)) return [];
    return items
      .map((a) => a.title || a.name || a.action_title || a.headline || "")
      .filter(Boolean);
  } catch {
    return [];
  }
}
