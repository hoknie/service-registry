package main

import (
	"strings"
	"testing"
)

func TestRules(t *testing.T) {
	t.Parallel()
	const f = module + "/internal/feature/"
	for name, tc := range map[string]struct {
		from, to, want string
	}{
		"pkg imports internal":             {module + "/pkg/ptr", module + "/internal/platform/apperr", "pkg/ must not import internal/"},
		"platform imports a feature":       {module + "/internal/platform/config", f + "catalog/types", "platform must not import features"},
		"feature against the graph":        {f + "catalog/services", f + "forge/services", "only the features below it"},
		"feature along the graph":          {f + "forge/services", f + "catalog/services", ""},
		"foreign adapter":                  {f + "knowledge/services", f + "forge/clients/forgeclient", "adapters of other features"},
		"own adapter":                      {f + "forge/services", f + "forge/clients/forgeclient", "its own adapters"},
		"types import services":            {f + "catalog/types", f + "catalog/services", "the domain of a feature"},
		"ports import repositories":        {f + "catalog/ports", f + "catalog/repositories", "ports of a feature"},
		"services import repositories":     {f + "catalog/services", f + "catalog/repositories", ""},
		"foreign repositories":             {f + "forge/services", f + "catalog/repositories", "used only by the services and use cases of its feature"},
		"use cases import repositories":    {f + "ingest/usecases", f + "ingest/repositories", ""},
		"services import use cases":        {f + "ingest/services", f + "ingest/usecases", ""},
		"use cases import services":        {f + "ingest/usecases", f + "ingest/services", "use cases must not import the services"},
		"repositories import use cases":    {f + "ingest/repositories", f + "ingest/usecases", "repositories must not import the services or use cases"},
		"foreign use cases":                {f + "deploy/services", f + "ingest/usecases", "used only by the services of its feature"},
		"presentation imports use cases":   {module + "/internal/presentation/http/handlers", f + "ingest/usecases", "used only by the services of its feature"},
		"handlers outside http":            {module + "/internal/app", module + "/internal/presentation/http/handlers", "for the HTTP layer only"},
		"jobs import app":                  {module + "/internal/presentation/jobs", module + "/internal/app", "only the console may import app"},
		"console imports app":              {module + "/internal/presentation/console", module + "/internal/app", ""},
		"presentation opens database":      {module + "/internal/presentation/http/handlers", module + "/internal/platform/postgres", "must not open the database"},
		"presentation opens a transaction": {module + "/internal/presentation/jobs", module + "/pkg/pgtx", "must not open the database"},
		"fiber in a feature":               {f + "links/services", "github.com/gofiber/fiber/v3", "features must not import presentation, app or Fiber"},
		"fiber in the http layer":          {module + "/internal/presentation/http", "github.com/gofiber/fiber/v3", ""},
		"unknown feature":                  {f + "billing/services", f + "catalog/types", "unknown feature"},
	} {
		got := strings.Join(check(tc.from, tc.to, false), "\n")
		if tc.want == "" && got != "" {
			t.Errorf("%s: unexpected problems:\n%s", name, got)
		}
		if tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("%s: problems %q, want %q", name, got, tc.want)
		}
	}
}

func TestFakesOnlyInTests(t *testing.T) {
	t.Parallel()
	const fake = module + "/pkg/testing/forgefake"
	for name, tc := range map[string]struct {
		from string
		test bool
		want bool
	}{
		"production code": {module + "/internal/feature/forge/clients/forgeclient", false, true},
		"test file":       {module + "/internal/feature/forge/clients/forgeclient", true, false},
		"package tests":   {module + "/tests", false, false},
		"testsupport":     {module + "/internal/testsupport", false, false},
		"another fake":    {module + "/pkg/testing/oidcfake", false, true},
	} {
		got := strings.Join(check(tc.from, fake, tc.test), "\n")
		if broken := strings.Contains(got, "pkg/testing is for tests only"); broken != tc.want {
			t.Errorf("%s: problems %q, want broken=%v", name, got, tc.want)
		}
	}
}
