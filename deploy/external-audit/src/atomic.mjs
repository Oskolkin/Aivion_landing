import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

export function sha256File(filePath) {
  const hash = crypto.createHash("sha256");
  hash.update(fs.readFileSync(filePath));
  return hash.digest("hex");
}

export function sha256Dir(dir) {
  const hash = crypto.createHash("sha256");
  const files = listFilesRecursive(dir).sort();
  for (const rel of files) {
    hash.update(rel);
    hash.update("\0");
    hash.update(fs.readFileSync(path.join(dir, rel)));
    hash.update("\0");
  }
  return hash.digest("hex");
}

export function listFilesRecursive(dir, base = dir, acc = []) {
  if (!fs.existsSync(dir)) return acc;
  for (const name of fs.readdirSync(dir)) {
    if (name === "." || name === "..") continue;
    const full = path.join(dir, name);
    const st = fs.lstatSync(full);
    if (st.isSymbolicLink()) continue;
    if (st.isDirectory()) listFilesRecursive(full, base, acc);
    else acc.push(path.relative(base, full).split(path.sep).join("/"));
  }
  return acc;
}

function rmrf(p) {
  fs.rmSync(p, { recursive: true, force: true });
}

function copyDir(src, dest) {
  fs.mkdirSync(dest, { recursive: true });
  for (const name of fs.readdirSync(src)) {
    const from = path.join(src, name);
    const to = path.join(dest, name);
    const st = fs.lstatSync(from);
    if (st.isDirectory()) copyDir(from, to);
    else fs.copyFileSync(from, to);
  }
}

/**
 * Atomically publish a staged revision.
 *
 * Layout:
 *   bundleRoot/revisions/rev-NNN/   (immutable history)
 *   bundleRoot/current              (copy of active revision; pointer file ACTIVE)
 *   publishedRoot/bundleId          (what Caddy serves) — atomic directory rename
 *
 * Switch algorithm (same filesystem):
 *   1. Move staging → revisions/rev-NNN
 *   2. Copy rev → publishedRoot/bundleId.next
 *   3. Rename publishedRoot/bundleId → bundleId.prev (if exists)
 *   4. Rename bundleId.next → bundleId
 *   5. Remove bundleId.prev
 *   6. Refresh bundleRoot/current copy + write ACTIVE marker
 *
 * On any failure after step 1, previous published dir remains untouched if steps 3–4 not done.
 */
export function atomicActivateRevision({
  bundleRoot,
  revisionName,
  revisionDir,
  publishedRoot,
  bundleId,
}) {
  const revisionsDir = path.join(bundleRoot, "revisions");
  const finalRev = path.join(revisionsDir, revisionName);
  fs.mkdirSync(revisionsDir, { recursive: true });
  fs.mkdirSync(publishedRoot, { recursive: true });

  if (fs.existsSync(finalRev)) {
    throw new Error(`revision already exists: ${revisionName}`);
  }

  fs.renameSync(revisionDir, finalRev);

  const pubLive = path.join(publishedRoot, bundleId);
  const pubNext = path.join(publishedRoot, `${bundleId}.next`);
  const pubPrev = path.join(publishedRoot, `${bundleId}.prev`);

  rmrf(pubNext);
  rmrf(pubPrev);
  copyDir(finalRev, pubNext);

  // Write ACTIVE marker inside next before swap.
  fs.writeFileSync(path.join(pubNext, ".revision"), revisionName, "utf8");

  if (fs.existsSync(pubLive)) {
    fs.renameSync(pubLive, pubPrev);
  }
  fs.renameSync(pubNext, pubLive);
  rmrf(pubPrev);

  const currentDir = path.join(bundleRoot, "current");
  rmrf(currentDir);
  copyDir(finalRev, currentDir);
  fs.writeFileSync(path.join(bundleRoot, "ACTIVE"), revisionName, "utf8");

  return { revisionPath: finalRev, publishedPath: pubLive, revisionName };
}

export function writeJson(filePath, data) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, JSON.stringify(data, null, 2) + "\n", "utf8");
}

export function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

export function safePublishOrKeep({ activateFn, onError }) {
  try {
    return { ok: true, result: activateFn() };
  } catch (err) {
    if (onError) onError(err);
    return { ok: false, error: String(err) };
  }
}
