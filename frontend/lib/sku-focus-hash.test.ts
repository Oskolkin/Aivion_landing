import assert from "node:assert/strict";
import {
  navigateToSKUFocusTarget,
  resolveSKUFocusFilterFromHash,
  resolveSKUFocusShowAllFromHash,
  resolveSKUFocusTabFromHash,
  scrollToSKUFocusSection,
  SKU_FOCUS_NAVIGATE_EVENT,
  SKU_FOCUS_SECTION_ID,
  targetToSKUFocusHash,
} from "./sku-focus-hash";

function run() {
  assert.equal(resolveSKUFocusTabFromHash("#sku-focus-no-sales"), "no_sales_with_stock");
  assert.equal(resolveSKUFocusTabFromHash("#sku-focus-stock"), "stock_risk");
  assert.equal(resolveSKUFocusTabFromHash("#sku-focus-surplus"), null);
  assert.equal(resolveSKUFocusTabFromHash(""), null);
  assert.equal(resolveSKUFocusTabFromHash("#other"), null);
  assert.equal(resolveSKUFocusTabFromHash("#sku-focus-stock?show=all"), "stock_risk", "resolves tab with show=all suffix");
  assert.equal(resolveSKUFocusTabFromHash("#sku-focus-stock&filter=all-risk"), "stock_risk", "resolves tab with filter suffix");
  assert.equal(
    resolveSKUFocusTabFromHash("#sku-focus-stock?show=all&filter=all-risk"),
    "stock_risk",
  );

  assert.equal(targetToSKUFocusHash("sku-focus-stock"), "#sku-focus-stock");
  assert.equal(targetToSKUFocusHash("#sku-focus-no-sales"), "#sku-focus-no-sales");
  assert.equal(targetToSKUFocusHash("unknown"), null);

  assert.equal(resolveSKUFocusShowAllFromHash("#sku-focus-stock"), false);
  assert.equal(resolveSKUFocusShowAllFromHash("#sku-focus-stock?show=all"), true);
  assert.equal(resolveSKUFocusShowAllFromHash("#sku-focus-stock&filter=all-risk"), true);
  assert.equal(resolveSKUFocusShowAllFromHash("#sku-focus-stock?show=all&filter=all-risk"), true);
  assert.equal(resolveSKUFocusShowAllFromHash(""), false);

  assert.equal(resolveSKUFocusFilterFromHash("#sku-focus-stock"), null);
  assert.equal(resolveSKUFocusFilterFromHash("#sku-focus-stock?filter=all-risk"), "all-risk");
  assert.equal(resolveSKUFocusFilterFromHash("#sku-focus-stock?show=all&filter=planning"), "planning");
  assert.equal(resolveSKUFocusFilterFromHash("#sku-focus-stock&filter=critical"), "critical");
  assert.equal(resolveSKUFocusFilterFromHash("#sku-focus-stock?filter=unknown"), null);

  const scrollCalls: unknown[] = [];
  const mockEl = {
    scrollIntoView: (opts: unknown) => {
      scrollCalls.push(opts);
    },
  };

  let hash = "";
  const navigateEvents: string[] = [];

  const g = globalThis as typeof globalThis & {
    window?: Window & typeof globalThis.window;
    document?: Document;
    requestAnimationFrame?: (cb: FrameRequestCallback) => number;
  };

  const origWindow = g.window;
  const origDocument = g.document;
  const origRaf = g.requestAnimationFrame;

  try {
    g.requestAnimationFrame = (cb) => {
      cb(0);
      return 1;
    };
    g.document = {
      getElementById: (id: string) => (id === SKU_FOCUS_SECTION_ID ? (mockEl as unknown as HTMLElement) : null),
    } as unknown as Document;

    g.window = {
      location: {
        get hash() {
          return hash;
        },
        set hash(value: string) {
          hash = value.startsWith("#") ? value : `#${value}`;
        },
      },
      addEventListener: () => {},
      dispatchEvent: (event: Event) => {
        if (event.type === SKU_FOCUS_NAVIGATE_EVENT) {
          navigateEvents.push((event as CustomEvent<{ hash: string }>).detail.hash);
        }
        return true;
      },
    } as unknown as Window & typeof globalThis.window;

    hash = "";
    navigateToSKUFocusTarget("sku-focus-no-sales");
    assert.equal(hash, "#sku-focus-no-sales", "updates hash for no-sales target");

    hash = "";
    navigateToSKUFocusTarget("sku-focus-stock");
    assert.equal(hash, "#sku-focus-stock", "updates hash for stock target");

    hash = "#sku-focus-no-sales";
    navigateEvents.length = 0;
    navigateToSKUFocusTarget("sku-focus-no-sales");
    assert.equal(navigateEvents.length, 1, "repeat click dispatches navigate event");
    assert.equal(navigateEvents[0], "#sku-focus-no-sales");

    hash = "#sku-focus-stock";
    navigateEvents.length = 0;
    navigateToSKUFocusTarget("sku-focus-stock");
    assert.equal(navigateEvents.length, 1, "repeat stock click dispatches navigate event");

    hash = "";
    navigateToSKUFocusTarget("sku-focus-stock", { showAll: true });
    assert.equal(hash, "#sku-focus-stock?show=all&filter=all-risk", "show=all and filter=all-risk appended");

    hash = "#sku-focus-stock";
    navigateToSKUFocusTarget("sku-focus-stock", { showAll: true });
    assert.equal(
      hash,
      "#sku-focus-stock?show=all&filter=all-risk",
      "navigating with showAll upgrades an existing plain hash",
    );

    scrollCalls.length = 0;
    scrollToSKUFocusSection();
    assert.equal(scrollCalls.length, 1, "scrolls to SKU focus section");
    assert.deepEqual(scrollCalls[0], { behavior: "smooth", block: "start" });
  } finally {
    g.window = origWindow;
    g.document = origDocument;
    g.requestAnimationFrame = origRaf;
  }

  console.log("sku-focus-hash tests passed");
}

run();
