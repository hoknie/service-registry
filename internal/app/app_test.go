package app_test

import (
	"testing"

	"svc-registry/internal/app"
	"svc-registry/internal/platform/config"
)

func TestNewWiresThePortsOfTheCatalog(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load[config.Config](config.Env{"DATABASE_URL": "postgres://registry@127.0.0.1:1/registry"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if !a.Catalog.Wired() {
		t.Fatal("catalog ports are not wired")
	}
}
