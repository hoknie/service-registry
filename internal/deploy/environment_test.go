package deploy

import (
	"errors"
	"testing"
)

func names(drop ...string) map[string]string {
	out := map[string]string{"en": "QA", "es": "QA", "ru": "Тест", "zh": "测试"}
	for _, d := range drop {
		delete(out, d)
	}
	return out
}

func TestEnvironmentRules(t *testing.T) {
	e, err := ValidateNewEnvironment(CreateEnvironment{Key: " QA ", Names: names()})
	if err != nil || e.Key != "qa" || e.Position != 0 || e.ID.Version() != 7 {
		t.Fatalf("%+v %v", e, err)
	}
	pos := int64(10_001)
	for name, tt := range map[string]struct {
		in   CreateEnvironment
		want error
	}{
		"bad key":      {CreateEnvironment{Key: "-x", Names: names()}, InvalidEnvironmentKey},
		"space":        {CreateEnvironment{Key: "a b", Names: names()}, InvalidEnvironmentKey},
		"no locale":    {CreateEnvironment{Key: "qa", Names: names("zh")}, InvalidEnvironmentNames},
		"no names":     {CreateEnvironment{Key: "qa"}, InvalidEnvironmentNames},
		"bad position": {CreateEnvironment{Key: "qa", Names: names(), Position: &pos}, InvalidPosition},
	} {
		if _, err := ValidateNewEnvironment(tt.in); !errors.Is(err, tt.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if c, err := ValidateEnvironmentChanges(UpdateEnvironment{}); err != nil || c.Names != nil || c.Position != nil {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestEnvironmentOrder(t *testing.T) {
	dir := []Environment{{Key: "production", Position: 10}, {Key: "staging", Position: 20}, {Key: "development", Position: 30}}
	keys := []string{"perf", "development", "alpha", "production"}
	SortEnvironments(keys, dir)
	if got := keys; got[0] != "production" || got[1] != "development" || got[2] != "alpha" || got[3] != "perf" {
		t.Fatal(got)
	}
}
