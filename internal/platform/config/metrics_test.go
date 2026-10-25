package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"svc-registry/internal/platform/config"
)

func TestMetricsDefaults(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.MetricsConfig](lookup())
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled || c.Token != "" || c.RefreshSecs != 60 {
		t.Fatalf("defaults %+v", c)
	}
}

func TestMetricsTokenIsCheckedWithoutLeaking(t *testing.T) {
	t.Parallel()
	for _, token := range []string{"short", "with spaces inside the token", strings.Repeat("a", 257)} {
		_, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL, "METRICS_ENABLED", "true", "METRICS_TOKEN", token))
		if got := vars(t, err); !reflect.DeepEqual(got, []string{"METRICS_TOKEN"}) {
			t.Fatalf("%q: %v", token, got)
		}
		if strings.Contains(err.Error(), token) {
			t.Fatalf("token leaked: %v", err)
		}
	}
	c, err := config.Load[config.MetricsConfig](lookup("METRICS_TOKEN", "0123456789abcdef"))
	if err != nil || c.Token != "0123456789abcdef" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestMetricsTokenFromFile(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "metrics")
	if err := os.WriteFile(file, []byte(strings.Repeat("x", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load[config.MetricsConfig](lookup("METRICS_ENABLED", "true", "METRICS_TOKEN", "file:"+file))
	if err != nil || c.Token != strings.Repeat("x", 32) || !c.Enabled {
		t.Fatalf("%+v %v", c, err)
	}
	_, err = config.Load[config.MetricsConfig](lookup("METRICS_TOKEN", "file:"+filepath.Join(t.TempDir(), "none")))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"METRICS_TOKEN"}) {
		t.Fatalf("%v", got)
	}
}

func TestMetricsRefreshRange(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1", "9", "3601", "often"} {
		_, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL, "METRICS_REFRESH_SECS", v))
		if got := vars(t, err); !reflect.DeepEqual(got, []string{"METRICS_REFRESH_SECS"}) {
			t.Fatalf("%s: %v", v, got)
		}
	}
	if _, err := config.Load[config.DbConfig](lookup("DATABASE_URL", dbURL, "METRICS_REFRESH_SECS", "1")); err != nil {
		t.Fatal(err)
	}
}
