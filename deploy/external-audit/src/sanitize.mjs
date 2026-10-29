/**
 * Strip secrets / auth / infra from captured JSON and text before publish.
 */

const SECRET_KEY_RE =
  /(password|passwd|secret|token|cookie|authorization|api[_-]?key|refresh|session|vless|xray|openai|bearer|signed[_-]?url|minio.*(key|secret)|private[_-]?key)/i;

const SECRET_VALUE_RE =
  /(sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{20,}|github_pat_[a-zA-Z0-9_]{20,}|eyJ[a-zA-Z0-9_-]{20,}\.[a-zA-Z0-9_-]+\.|Bearer\s+[A-Za-z0-9\-._~+/]+=*|session_token=|vless:\/\/|ss:\/\/)/i;

const REDACTED = "[REDACTED]";

export function sanitizeValue(value, key = "") {
  if (value == null) return value;
  if (typeof value === "string") {
    if (SECRET_KEY_RE.test(key) || SECRET_VALUE_RE.test(value)) return REDACTED;
    return value.replace(SECRET_VALUE_RE, REDACTED);
  }
  if (typeof value === "number" || typeof value === "boolean") return value;
  if (Array.isArray(value)) return value.map((v) => sanitizeValue(v, key));
  if (typeof value === "object") return sanitizeObject(value);
  return value;
}

export function sanitizeObject(input) {
  if (input == null || typeof input !== "object") return sanitizeValue(input);
  if (Array.isArray(input)) return input.map((v) => sanitizeValue(v));
  const out = {};
  for (const [k, v] of Object.entries(input)) {
    if (SECRET_KEY_RE.test(k)) {
      out[k] = REDACTED;
      continue;
    }
    out[k] = sanitizeValue(v, k);
  }
  return out;
}

export function sanitizeText(text) {
  if (!text) return "";
  return String(text).replace(SECRET_VALUE_RE, REDACTED);
}

export function scanForSecrets(payload) {
  const text = typeof payload === "string" ? payload : JSON.stringify(payload);
  const hits = [];
  const re = new RegExp(SECRET_VALUE_RE.source, "gi");
  let m;
  while ((m = re.exec(text)) !== null) {
    hits.push({ index: m.index, sample: m[0].slice(0, 12) + "…" });
    if (hits.length >= 20) break;
  }
  // Also flag raw password-looking keys in JSON dumps
  if (/"password"\s*:\s*"[^"]+"/i.test(text)) {
    hits.push({ index: -1, sample: "password_field…" });
  }
  return hits;
}
