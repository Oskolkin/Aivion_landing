import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import sharp from "sharp";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const out = path.resolve(__dirname, "../../backend/debug/ai-chat-audit/fixtures/control.webp");

const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360">
  <rect width="100%" height="100%" fill="#f5f8ff"/>
  <text x="40" y="60" font-size="20" font-family="Arial">TITLE: Stock Report</text>
  <text x="40" y="100" font-size="18" font-family="Arial">TABLE: SKU | Orders | Revenue</text>
  <text x="40" y="140" font-size="18" font-family="Arial">801 | 12 | 48317</text>
  <text x="40" y="180" font-size="22" font-family="Arial" fill="#111111">WEBP_CONTROL_77324</text>
  <text x="40" y="220" font-size="18" font-family="Arial">VALUE_ON_CHART=48317</text>
</svg>`;

await sharp(Buffer.from(svg)).webp({ lossless: true }).toFile(out);
// Append ASCII marker for local extractor fallback without breaking RIFF size for decode of leading image.
const bytes = fs.readFileSync(out);
fs.writeFileSync(out, Buffer.concat([bytes, Buffer.from("\nWEBP_CONTROL_77324\n")]));
console.log("wrote", out, fs.statSync(out).size);
