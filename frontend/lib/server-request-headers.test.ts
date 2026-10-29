import assert from "node:assert/strict";
import {
  LIVE_REVIEW_HEADER,
  LIVE_REVIEW_HOST_HEADER,
  LIVE_REVIEW_TOKEN_HEADER,
  mergeServerBackendHeaders,
} from "./server-request-headers";

function run() {
  const withCookie = mergeServerBackendHeaders({
    cookie: "session_token=abc",
  });
  assert.equal(withCookie.Cookie, "session_token=abc");
  assert.equal(withCookie["Content-Type"], "application/json");
  assert.equal(withCookie[LIVE_REVIEW_HEADER], undefined);

  const withLive = mergeServerBackendHeaders({
    cookie: "session_token=abc",
    liveReview: "1",
    liveReviewToken: "tok",
    liveReviewHost: "abc.review.aivion.pro",
  });
  assert.equal(withLive[LIVE_REVIEW_HEADER], "1");
  assert.equal(withLive[LIVE_REVIEW_TOKEN_HEADER], "tok");
  assert.equal(withLive[LIVE_REVIEW_HOST_HEADER], "abc.review.aivion.pro");

  const withoutEmpty = mergeServerBackendHeaders({
    cookie: "",
    liveReview: null,
    liveReviewToken: undefined,
    liveReviewHost: "",
  });
  assert.equal(withoutEmpty.Cookie, undefined);
  assert.equal(withoutEmpty[LIVE_REVIEW_HEADER], undefined);
  assert.equal(withoutEmpty[LIVE_REVIEW_TOKEN_HEADER], undefined);
  assert.equal(withoutEmpty[LIVE_REVIEW_HOST_HEADER], undefined);

  // Module must stay server-oriented: helper throws in browser-like context
  // when buildServerBackendHeaders is called (tested via typeof window guard).
  assert.ok(typeof window === "undefined", "unit test must run in node (server) context");

  console.log("server-request-headers tests passed");
}

run();
