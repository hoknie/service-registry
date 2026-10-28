import assert from "node:assert/strict";
import { test } from "node:test";

import {
  agentConfig,
  filtersFrom,
  gatewayName,
  listQuery,
  mcpEndpoint,
  placeFrom,
  placeHref,
  serverListBody,
  stateTone,
} from "../src/lib/mcp.ts";
import { slugOf } from "../src/lib/slug.ts";

test("places and filters live in the address", () => {
  assert.deepEqual(placeFrom("folder=f1&q=x"), { folder: "f1", space: null });
  assert.deepEqual(filtersFrom("q=jira&tag=a&tag=b&state=ok"), {
    q: "jira",
    tags: ["a", "b"],
    state: "ok",
  });
  assert.equal(
    placeHref(
      "ru",
      { folder: null, space: "global" },
      { q: "x", tags: ["t"], state: "failing" },
    ),
    "/ru/mcp?space=global&q=x&tag=t&state=failing",
  );
  assert.equal(placeHref("en", { folder: null, space: null }), "/en/mcp");
});

test("list queries ask for the level unless searching", () => {
  assert.equal(
    listQuery(
      { folder: null, space: "global" },
      { q: "", tags: [], state: "" },
    ),
    "/v1/mcp/servers?organization_id=global&folder_id=root",
  );
  assert.equal(
    listQuery(
      { folder: "f1", space: null },
      { q: "a", tags: [], state: "ok" },
      "c",
    ),
    "/v1/mcp/servers?folder_id=f1&q=a&state=ok&cursor=c",
  );
});

test("gateway names, tones and connection hints", () => {
  assert.equal(gatewayName("jira", "search"), "jira__search");
  assert.equal(stateTone("failing"), "danger");
  assert.equal(stateTone("unknown"), "neutral");
  assert.equal(
    mcpEndpoint("http://localhost:8080", "https://registry.example/"),
    "https://registry.example/api/mcp",
  );
  assert.equal(
    mcpEndpoint("http://localhost:8080", ""),
    "http://localhost:8080/api/mcp",
  );
  assert.match(
    agentConfig("https://r/api/mcp", "claude"),
    /claude mcp add --transport http registry https:\/\/r\/api\/mcp/,
  );
  assert.equal(
    JSON.parse(agentConfig("https://r/api/mcp", "json")).mcpServers.registry
      .url,
    "https://r/api/mcp",
  );
  assert.deepEqual(serverListBody(["a", "a", "b"]), { servers: ["a", "b"] });
  assert.deepEqual(serverListBody(null), { servers: null });
});

test("slugs come from names, with a length limit", () => {
  assert.equal(slugOf("Джира Трекер", 32), "dzhira-treker");
  assert.equal(slugOf("x".repeat(40), 32).length, 32);
});
