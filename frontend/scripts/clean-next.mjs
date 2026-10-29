import { rmSync } from "node:fs";
import { join } from "node:path";

const dir = join(process.cwd(), ".next");
rmSync(dir, { recursive: true, force: true });
console.log("Removed", dir);
