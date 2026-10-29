import assert from "node:assert/strict";
import { ACTION_ASK_MESSAGE } from "@/components/actions/action-cta-buttons";

assert.match(ACTION_ASK_MESSAGE, /Помоги разобраться с этим действием/);
assert.match(ACTION_ASK_MESSAGE, /пошаговый план выполнения/);
assert.doesNotMatch(ACTION_ASK_MESSAGE, /Explain this action/i);

console.log("actions-cta.test.tsx passed");
