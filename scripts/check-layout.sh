#!/usr/bin/env bash
set -euo pipefail
GO="${GO:-go}"
mod="svc-registry"

"$GO" list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... | awk -v mod="$mod" '
function feature(p,   rest, n, parts) {
  if (index(p, mod "/internal/feature/") != 1) return ""
  rest = substr(p, length(mod "/internal/feature/") + 1)
  n = split(rest, parts, "/")
  return parts[1]
}
function layer(p,   rest, cut) {
  if (index(p, mod "/internal/feature/") != 1) return ""
  rest = substr(p, length(mod "/internal/feature/") + 1)
  cut = index(rest, "/")
  if (cut == 0) return "domain"
  rest = substr(rest, cut + 1)
  if (rest == "service" || rest == "repository") return rest
  return "adapter"
}
function adapter(p) { return layer(p) == "adapter" }
function httpPart(p) { return under(p, "internal/presentation/http/handlers") || under(p, "internal/presentation/http/requests") || under(p, "internal/presentation/http/responses") || under(p, "internal/presentation/http/middleware") || under(p, "internal/presentation/http/mcp") }
function under(p, prefix) { return p == mod "/" prefix || index(p, mod "/" prefix "/") == 1 }
BEGIN {
  split("access: catalog:access ingest:catalog,access links:catalog,access forge:catalog,access deploy:ingest,forge,catalog,access knowledge:forge,catalog,access", rows, " ")
  for (i in rows) {
    split(rows[i], kv, ":")
    known[kv[1]] = 1
    n = split(kv[2], deps, ",")
    for (j = 1; j <= n; j++) if (deps[j] != "") ok[kv[1], deps[j]] = 1
  }
  bad = 0
}
function fail(rule, from, to) { print "layout: " rule ": " from " -> " to; bad = 1 }
{
  pkg = $1
  for (i = 2; i <= NF; i++) {
    imp = $i
    if (under(pkg, "pkg") && under(imp, "internal")) fail("pkg/ must not import internal/", pkg, imp)
    if (under(pkg, "internal/platform") && (under(imp, "internal/feature") || under(imp, "internal/presentation") || under(imp, "internal/app")))
      fail("platform must not import features, presentation or app", pkg, imp)
    f = feature(pkg)
    if (f != "") {
      if (!(f in known)) fail("unknown feature (add it to the graph)", pkg, imp)
      if (under(imp, "internal/presentation") || under(imp, "internal/app") || index(imp, "github.com/gofiber/") == 1)
        fail("features must not import presentation, app or Fiber", pkg, imp)
      g = feature(imp)
      if (g != "" && g != f) {
        if (!((f, g) in ok)) fail("feature " f " may not import feature " g, pkg, imp)
        if (adapter(imp)) fail("features must not import adapters of other features", pkg, imp)
      }
      if (g == f && !adapter(pkg) && adapter(imp)) fail("a feature must not import its own adapters (app wires them)", pkg, imp)
      if (g == f && layer(pkg) == "domain" && (layer(imp) == "service" || layer(imp) == "repository"))
        fail("the domain of a feature must not import its service or repositories", pkg, imp)
      if (g == f && layer(pkg) == "repository" && layer(imp) == "service")
        fail("repositories must not import the service", pkg, imp)
    }
    if (layer(imp) == "repository" && !(feature(pkg) == feature(imp) && (layer(pkg) == "service" || layer(pkg) == "repository")))
      fail("a repository is used only by the service of its feature", pkg, imp)
    if (httpPart(imp) && !under(pkg, "internal/presentation/http"))
      fail("handlers, requests, responses, middleware and mcp are for the HTTP layer only", pkg, imp)
    if (under(pkg, "internal/presentation") && !under(pkg, "internal/presentation/console") && under(imp, "internal/app"))
      fail("only the console may import app", pkg, imp)
    if (under(pkg, "internal/presentation") && adapter(imp))
      fail("presentation must not import adapters of features", pkg, imp)
    if (index(imp, "github.com/gofiber/") == 1 && !under(pkg, "internal/presentation/http") && !under(pkg, "internal/app") && !under(pkg, "tests"))
      fail("Fiber is allowed only in presentation/http", pkg, imp)
  }
}
END { exit bad }
'
