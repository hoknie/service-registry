package links

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestVarRules(t *testing.T) {
	if v, err := ValidateVars(nil); err != nil || v == nil {
		t.Fatal(v, err)
	}
	if _, err := ValidateVars(map[string]string{"grafana_org": "7", "team.name": strings.Repeat("x", 1024)}); err != nil {
		t.Fatal(err)
	}
	many := map[string]string{}
	for i := range 33 {
		many["k"+strings.Repeat("a", i)] = "v"
	}
	for name, bad := range map[string]map[string]string{
		"key upper":   {"Team": "x"},
		"key empty":   {"": "x"},
		"key dot end": {"a.": "x"},
		"long value":  {"a": strings.Repeat("x", 1025)},
		"control":     {"a": "x\ny"},
		"too many":    many,
	} {
		if _, err := ValidateVars(bad); err != InvalidVars {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestNearestVarWins(t *testing.T) {
	org, folder := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	got := EffectiveVars([][]Var{
		{},
		{{Key: "grafana_org", Value: "7", NodeID: folder}},
		{{Key: "grafana_org", Value: "1", NodeID: org}, {Key: "a", Value: "x", NodeID: org}},
	})
	if len(got) != 2 || got[0].Key != "a" || got[1].Value != "7" || got[1].NodeID != folder || !got[1].Inherited {
		t.Fatalf("%+v", got)
	}
}
