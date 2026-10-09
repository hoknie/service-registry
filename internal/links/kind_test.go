package links

import (
	"errors"
	"testing"
)

func names(overrides map[string]string) map[string]string {
	out := map[string]string{"en": "Logs", "es": "Registros", "ru": "Логи", "zh": "日志"}
	for k, v := range overrides {
		if v == "" {
			delete(out, k)
		} else {
			out[k] = v
		}
	}
	return out
}

func TestNewKindNormalizesTheKeyAndAppliesDefaults(t *testing.T) {
	k, err := ValidateNewKind(CreateKind{Key: " Grafana-Business ", Names: names(map[string]string{"en": "  Business  "})})
	if err != nil {
		t.Fatal(err)
	}
	if k.Key != "grafana-business" || k.Icon != "link" || k.Position != 0 || k.Names["en"] != "Business" {
		t.Fatalf("%+v", k)
	}
	if k.ID.Version() != 7 {
		t.Fatal("UUID v7")
	}
}

func TestKindRules(t *testing.T) {
	icon, badIcon := "dashboard", "rocket"
	pos, badPos := int64(10_000), int64(10_001)
	tests := []struct {
		name string
		in   CreateKind
		want error
	}{
		{"ok", CreateKind{Key: "grafana", Names: names(nil), Icon: &icon, Position: &pos}, nil},
		{"empty key", CreateKind{Key: " ", Names: names(nil)}, InvalidKindKey},
		{"leading dash", CreateKind{Key: "-x", Names: names(nil)}, InvalidKindKey},
		{"underscore", CreateKind{Key: "a_b", Names: names(nil)}, InvalidKindKey},
		{"missing locale", CreateKind{Key: "x", Names: names(map[string]string{"es": "", "zh": ""})}, InvalidKindNames},
		{"extra locale", CreateKind{Key: "x", Names: map[string]string{"en": "a", "es": "a", "ru": "a", "de": "a"}}, InvalidKindNames},
		{"blank name", CreateKind{Key: "x", Names: names(map[string]string{"ru": "   "})}, InvalidKindNames},
		{"control", CreateKind{Key: "x", Names: names(map[string]string{"ru": "a\nb"})}, InvalidKindNames},
		{"no names", CreateKind{Key: "x"}, InvalidKindNames},
		{"icon", CreateKind{Key: "x", Names: names(nil), Icon: &badIcon}, InvalidIcon},
		{"position", CreateKind{Key: "x", Names: names(nil), Position: &badPos}, InvalidPosition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateNewKind(tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestKindChangesOnlyTouchGivenFields(t *testing.T) {
	c, err := ValidateKindChanges(UpdateKind{})
	if err != nil || c.Names != nil || c.Icon != nil || c.Position != nil {
		t.Fatalf("%+v %v", c, err)
	}
	icon := "docs"
	if c, err = ValidateKindChanges(UpdateKind{Icon: &icon, Names: names(nil)}); err != nil || *c.Icon != "docs" || c.Names["ru"] != "Логи" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := ValidateKindChanges(UpdateKind{Names: map[string]string{"en": "x"}}); err != InvalidKindNames {
		t.Fatal(err)
	}
}
