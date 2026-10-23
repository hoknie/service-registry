import assert from "node:assert/strict";
import { test } from "node:test";

import { appHost, appName, claudeCommand, consentError, consentParams, isLoopbackRedirect, loginHref, reasonText } from "../src/lib/oauthApps.ts";

test("the login address keeps the query of the protected page", () => {
  assert.equal(loginHref("ru", "/ru/oauth/authorize", "?request=abc"), "/ru/login?next=%2Fru%2Foauth%2Fauthorize%3Frequest%3Dabc");
  assert.equal(loginHref("zh", "/zh/admin/users", ""), "/zh/login?next=%2Fzh%2Fadmin%2Fusers");
});

test("the Claude Code command has a header only with a token", () => {
  assert.equal(claudeCommand("https://r.example", null), "claude mcp add --transport http registry https://r.example/api/mcp");
  assert.equal(
    claudeCommand("https://r.example", "svcp_x"),
    'claude mcp add --transport http registry https://r.example/api/mcp \\\n  --header "Authorization: Bearer svcp_x"',
  );
});

test("consent parameters and known error codes", () => {
  assert.deepEqual(consentParams("?request=abc"), { request: "abc", error: null, hint: null });
  assert.deepEqual(consentParams("?error=oauth.client_not_allowed&hint=metadata_hosts&host=app.example").hint, {
    kind: "metadata_hosts",
    host: "app.example",
  });
  assert.equal(consentParams("?error=oauth.invalid_client&hint=other&host=app.example").hint, null);
  assert.equal(consentParams("?error=oauth.invalid_client&hint=metadata_disabled").hint, null);
  assert.deepEqual(consentParams("?hint=metadata_unavailable&host=app.example&reason=status_404").hint, {
    kind: "metadata_unavailable",
    host: "app.example",
    reason: "status_404",
  });
  assert.deepEqual(consentParams("?hint=metadata_invalid&host=app.example&reason=has_secret").hint?.reason, "has_secret");
  assert.equal(consentParams("?hint=metadata_invalid&host=app.example&reason=timeout").hint, null);
  assert.equal(consentParams("?hint=metadata_unavailable&host=app.example&reason=status_abc").hint, null);
  assert.equal(consentParams("?hint=metadata_unavailable&host=app.example").hint, null);
  assert.equal(consentParams("?hint=metadata_hosts&host=app.example&reason=timeout").hint?.reason, undefined);
  assert.equal(consentError("oauth.invalid_redirect_uri"), "oauth.invalid_redirect_uri");
  assert.equal(consentError("javascript:alert(1)"), null);
  assert.equal(consentError(null), null);
  assert.equal(appName({ name: null }, "Unnamed app"), "Unnamed app");
  assert.equal(appName({ name: "Claude Code" }, "Unnamed app"), "Claude Code");
});

test("clients described by a document", () => {
  assert.equal(consentError("oauth.client_metadata_unavailable"), "oauth.client_metadata_unavailable");
  assert.equal(consentError("oauth.client_not_allowed"), "oauth.client_not_allowed");
  assert.equal(appHost({ redirect_host: "localhost:3000", metadata_host: "app.example" }), "app.example");
  assert.equal(appHost({ redirect_host: "localhost:3000", metadata_host: null }), "localhost:3000");
  assert.equal(isLoopbackRedirect("http://localhost:3000/cb"), true);
  assert.equal(isLoopbackRedirect("http://[::1]:3000/cb"), true);
  assert.equal(isLoopbackRedirect("http://127.0.0.1/cb"), true);
  assert.equal(isLoopbackRedirect("https://app.example/cb"), false);
  assert.equal(isLoopbackRedirect("com.example.app:/cb"), false);
  assert.equal(isLoopbackRedirect(undefined), false);
});

test("refusal reasons are translated, statuses carry their code", () => {
  const reasons = { timeout: "no answer", status: "status {status}" };
  assert.equal(reasonText(reasons, "timeout"), "no answer");
  assert.equal(reasonText(reasons, "status_502"), "status 502");
  assert.equal(reasonText(reasons, "dns"), "dns");
});
