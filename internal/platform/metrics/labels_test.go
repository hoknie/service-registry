package metrics_test

import (
	"errors"
	"testing"

	"svc-registry/internal/platform/metrics"
)

func TestLabelNormalization(t *testing.T) {
	t.Parallel()
	cases := []struct{ got, want string }{
		{metrics.Method("GET"), "GET"},
		{metrics.Method("PROPFIND"), "other"},
		{metrics.ResultClass(204, nil), "2xx"},
		{metrics.ResultClass(502, nil), "5xx"},
		{metrics.ResultClass(0, errors.New("timeout")), "error"},
		{metrics.ResultClass(200, errors.New("late")), "error"},
		{metrics.Code("auth.invalid_project_key"), "auth.invalid_project_key"},
		{metrics.Code(""), ""},
		{metrics.Code("Something went wrong"), "other"},
		{metrics.Code("plain"), "other"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("%d: got %q, want %q", i, c.got, c.want)
		}
	}
}

func TestFromPairs(t *testing.T) {
	t.Parallel()
	got := metrics.FromPairs(map[string]int64{"nodes:project": 12, "clusters:agent/ok": 1, "groups": 3, "nodes:a/b": 9, "other:x": 1},
		map[string]metrics.Shape{
			"nodes":    {Name: "svcr_catalog_nodes", Labels: []string{"kind"}},
			"clusters": {Name: "svcr_clusters", Labels: []string{"mode", "state"}},
			"groups":   {Name: "svcr_groups"},
		})
	if len(got) != 3 {
		t.Fatalf("%+v", got)
	}
	if got[0].Name != "svcr_clusters" || got[0].Labels["mode"] != "agent" || got[0].Labels["state"] != "ok" || got[0].Value != 1 {
		t.Fatalf("%+v", got[0])
	}
	if got[1].Name != "svcr_groups" || len(got[1].Labels) != 0 || got[1].Value != 3 {
		t.Fatalf("%+v", got[1])
	}
	if got[2].Labels["kind"] != "project" || got[2].Value != 12 {
		t.Fatalf("%+v", got[2])
	}
}
