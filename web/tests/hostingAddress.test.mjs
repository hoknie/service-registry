import assert from "node:assert/strict";
import { test } from "node:test";

import { guessHosting, ownerUrlOf, parseOwnerUrl, sourceApiUrl } from "../src/lib/hostingAddress.ts";

const sourceCases = [
  ["github", "https://github.com/acme/api", "https://api.github.com"],
  ["github", "https://github.example.com/acme/api", "https://github.example.com/api/v3"],
  ["gitlab", "https://gitlab.example.com/platform/api", "https://gitlab.example.com/api/v4"],
  ["gitlab", "https://gitlab.com/acme/platform/api.git", "https://gitlab.com/api/v4"],
  ["gitea", "https://git.example.com/team/api", "https://git.example.com/api/v1"],
  ["forgejo", "https://forgejo.example.com:8443/team/api/", "https://forgejo.example.com:8443/api/v1"],
  ["forgejo", "http://localhost:3000/team/api", "http://localhost:3000/api/v1"],
  ["gitea", "http://git.example.com/team/api", null],
  ["gitea", "https://git.example.com/api", null],
  ["gitea", "https://git.example.com/team/api?x=1", null],
  ["gitea", "https://user@git.example.com/team/api", null],
  ["", "https://git.example.com/team/api", null],
];

test("source api address follows the server rules", () => {
  for (const [kind, url, want] of sourceCases) assert.equal(sourceApiUrl(kind, url), want, `${kind} ${url}`);
});

test("hosting kind is guessed only for public hosts", () => {
  assert.equal(guessHosting("https://github.com/acme/api"), "github");
  assert.equal(guessHosting("https://gitlab.com/acme"), "gitlab");
  assert.equal(guessHosting("https://git.example.com/acme"), null);
  assert.equal(guessHosting("nonsense"), null);
});

const ownerCases = [
  ["https://forgejo.example.com/journey-crate", "forgejo", { kind: null, apiUrl: "https://forgejo.example.com/api/v1", ownerPath: "journey-crate" }],
  ["https://gitlab.com/acme/platform/", "", { kind: "gitlab", apiUrl: "https://gitlab.com", ownerPath: "acme/platform" }],
  ["https://gitlab.example.com/platform/team", "gitlab", { kind: null, apiUrl: "https://gitlab.example.com", ownerPath: "platform/team" }],
  ["https://github.com/acme-inc", "", { kind: "github", apiUrl: "https://api.github.com", ownerPath: "acme-inc" }],
  ["https://ghe.example.com/acme", "github", { kind: null, apiUrl: "https://ghe.example.com/api/v3", ownerPath: "acme" }],
  ["https://git.example.com/team", "gitea", { kind: null, apiUrl: "https://git.example.com/api/v1", ownerPath: "team" }],
  ["https://git.example.com/team", "", { kind: null, apiUrl: null, ownerPath: "team" }],
];

test("owner address gives kind, api and owner", () => {
  for (const [url, kind, want] of ownerCases) assert.deepEqual(parseOwnerUrl(url, kind), want, url);
  assert.equal(parseOwnerUrl("https://git.example.com/", "gitea"), null);
  assert.equal(parseOwnerUrl("ftp://git.example.com/team", "gitea"), null);
});

test("owner address is rebuilt from saved values", () => {
  assert.equal(ownerUrlOf("https://api.github.com", "acme"), "https://github.com/acme");
  assert.equal(ownerUrlOf("https://forgejo.example.com/api/v1", "journey-crate"), "https://forgejo.example.com/journey-crate");
  assert.equal(ownerUrlOf("https://gitlab.com", "acme/platform"), "https://gitlab.com/acme/platform");
});
