import { sparklineNumericValue } from "@/lib/sparkline-value";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(sparklineNumericValue(null) === null, "null stays null");
  assert(sparklineNumericValue(undefined) === null, "undefined stays null");
  assert(sparklineNumericValue(0) === 0, "zero stays zero");
  assert(sparklineNumericValue(5) === 5, "number preserved");
  console.log("sparkline-value.test.ts: ok");
}

run();
