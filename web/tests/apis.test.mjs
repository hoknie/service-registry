import assert from "node:assert/strict";
import { test } from "node:test";

import { filterOperations, groupByTag, importedType, localType, operationAnchor, rememberAnchor, schemaAnchor, takeAnchor, viewParam, withView } from "../src/lib/apis.ts";

const op = (method, path, extra = {}) => ({ key: `${method} ${path}`, method, path, operation_id: null, summary: null, tags: [], ...extra });

test("anchors of operations and schemas", () => {
  assert.equal(operationAnchor(op("GET", "/pets/{id}")), "op-get-pets-id");
  assert.equal(schemaAnchor("Pet"), "schema-Pet");
  assert.equal(schemaAnchor("pets.v1/Pet Item"), "schema-pets.v1-Pet-Item");
});

test("filter keeps operations by path, summary or operation id", () => {
  const ops = [op("GET", "/pets", { summary: "List pets" }), op("POST", "/orders"), op("GET", "/x", { operation_id: "getOrder" })];
  assert.deepEqual(filterOperations(ops, "ORDER").map((o) => o.path), ["/orders", "/x"]);
  assert.equal(filterOperations(ops, "  ").length, 3);
  assert.equal(filterOperations(ops, "none").length, 0);
});

test("groups follow the tag order and put untagged operations last", () => {
  const ops = [op("GET", "/a"), op("GET", "/b", { tags: ["store"] }), op("GET", "/c", { tags: ["pets", "store"] })];
  const groups = groupByTag([{ name: "pets" }, { name: "store" }, { name: "empty" }], ops);
  assert.deepEqual(groups.map((g) => [g.tag, g.operations.map((o) => o.path)]), [["pets", ["/c"]], ["store", ["/b", "/c"]], [null, ["/a"]]]);
});

test("view lives in the address", () => {
  assert.equal(viewParam("?view=tree"), "tree");
  assert.equal(viewParam("?view=bogus"), "api");
  assert.equal(withView("/doc/acme/api/openapi.yaml?branch=x", "source"), "/doc/acme/api/openapi.yaml?branch=x&view=source");
  assert.equal(withView("/en/catalog?node=1&view=tree#op-get-a", "api"), "/en/catalog?node=1#op-get-a");
});

test("proto types resolve to definitions of the same file", () => {
  const known = new Set(["pets.v1.Pet", "pets.v1.Pet.Toy"]);
  assert.equal(localType(".pets.v1.Pet", "pets.v1", known), "pets.v1.Pet");
  assert.equal(localType("Pet", "pets.v1", known), "pets.v1.Pet");
  assert.equal(localType("common.v1.Money", "pets.v1", known), null);
});

test("proto types of imported files point to their file", () => {
  const files = { "common.v1.Money": "proto/common/v1/money.proto" };
  assert.deepEqual(importedType(".common.v1.Money", files), { name: "common.v1.Money", path: "proto/common/v1/money.proto" });
  assert.equal(importedType(".google.protobuf.Timestamp", files), null);
  assert.equal(importedType("string", undefined), null);
});

test("an anchor survives a move to another file once", () => {
  rememberAnchor("type-common.v1.Money");
  assert.equal(takeAnchor(""), "type-common.v1.Money");
  assert.equal(takeAnchor(""), null);
  assert.equal(takeAnchor("#type-a%20b"), "type-a b");
});
