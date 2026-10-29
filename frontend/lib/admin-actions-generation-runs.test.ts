import assert from "node:assert/strict";

import { buildAdminRunFiltersQuery, parseAdminRunFilters } from "@/lib/admin-actions-generation-runs";

const parsed = parseAdminRunFilters({
  seller_id: "12",
  status: "completed",
  final_status: "completed_partial",
  metric_date: "2026-06-24",
});
assert.equal(parsed.seller_id, 12);
assert.equal(parsed.status, "completed");
assert.equal(parsed.final_status, "completed_partial");
assert.equal(parsed.metric_date, "2026-06-24");

const parsedInvalid = parseAdminRunFilters({
  seller_id: "abc",
  status: " ",
});
assert.equal(parsedInvalid.seller_id, undefined);
assert.equal(parsedInvalid.status, undefined);

const query = buildAdminRunFiltersQuery({
  seller_id: 7,
  status: "completed",
  final_status: "completed",
  metric_date: "2026-06-24",
});
assert.equal(query, "?seller_id=7&status=completed&final_status=completed&metric_date=2026-06-24");

const queryEmpty = buildAdminRunFiltersQuery({ seller_id: -1, status: " " });
assert.equal(queryEmpty, "");

console.log("admin-actions-generation-runs.test.ts: ok");
