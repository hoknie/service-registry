package types

import (
	"errors"
	"testing"

	"svc-registry/pkg/ptr"
)

func names(overrides map[string]string) map[string]*string {
	out := map[string]*string{"en": ptr.To("Logs"), "es": ptr.To("Registros"), "ru": ptr.To("Логи"), "zh": ptr.To("日志")}
	for k, v := range overrides {
		if v == "" {
			delete(out, k)
		} else {
			out[k] = ptr.To(v)
		}
	}
	return out
}

func TestNewKindNormalizesTheKeyAndAppliesDefaults(t *testing.T) {
	k, err := ValidateNewKind(CreateKind{Key: " Grafana-Business ", Names: map[string]*string{"en": ptr.To("  Business  ")}})
	if err != nil {
		t.Fatal(err)
	}
	if k.Key != "grafana-business" || k.Icon != "link" || k.Position != nil || k.IconURL != nil || k.Names["en"] != "Business" {
		t.Fatalf("%+v", k)
	}
	if k.ID.Version() != 7 {
		t.Fatal("UUID v7")
	}
}

func TestNamesAreCopiedFromTheFirstGiven(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]*string
		want Names
	}{
		{"one", map[string]*string{"ru": ptr.To("Пайплайн"), "en": ptr.To("")}, Names{"en": "Пайплайн", "es": "Пайплайн", "ru": "Пайплайн", "zh": "Пайплайн"}},
		{"first in order", map[string]*string{"es": ptr.To("Registros"), "ru": ptr.To("Логи")}, Names{"en": "Registros", "es": "Registros", "ru": "Логи", "zh": "Registros"}},
		{"null skipped", map[string]*string{"en": nil, "zh": ptr.To(" 日志 ")}, Names{"en": "日志", "es": "日志", "ru": "日志", "zh": "日志"}},
		{"all", names(nil), Names{"en": "Logs", "es": "Registros", "ru": "Логи", "zh": "日志"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateNames(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range Locales {
				if got[l] != tt.want[l] {
					t.Fatalf("%s: got %q, want %q", l, got[l], tt.want[l])
				}
			}
		})
	}
}

func TestKindRules(t *testing.T) {
	pos, badPos := int64(10_000), int64(10_001)
	tests := []struct {
		name string
		in   CreateKind
		want error
	}{
		{"ok", CreateKind{Key: "grafana", Names: names(nil), Icon: ptr.To("git-pull-request"), IconURL: ptr.To("https://ci.example/favicon.png"), Position: &pos}, nil},
		{"empty key", CreateKind{Key: " ", Names: names(nil)}, InvalidKindKey},
		{"leading dash", CreateKind{Key: "-x", Names: names(nil)}, InvalidKindKey},
		{"underscore", CreateKind{Key: "a_b", Names: names(nil)}, InvalidKindKey},
		{"no locale", CreateKind{Key: "x", Names: map[string]*string{"en": ptr.To("  "), "ru": nil}}, InvalidKindNames},
		{"unknown locale", CreateKind{Key: "x", Names: map[string]*string{"de": ptr.To("Protokolle")}}, InvalidKindNames},
		{"extra locale", CreateKind{Key: "x", Names: map[string]*string{"en": ptr.To("a"), "de": ptr.To("a")}}, InvalidKindNames},
		{"control", CreateKind{Key: "x", Names: names(map[string]string{"ru": "a\nb"})}, InvalidKindNames},
		{"too long", CreateKind{Key: "x", Names: names(map[string]string{"ru": string(make([]rune, 101))})}, InvalidKindNames},
		{"no names", CreateKind{Key: "x"}, InvalidKindNames},
		{"icon spaces", CreateKind{Key: "x", Names: names(nil), Icon: ptr.To("Chart Line")}, InvalidIcon},
		{"icon double dash", CreateKind{Key: "x", Names: names(nil), Icon: ptr.To("chart--line")}, InvalidIcon},
		{"icon http", CreateKind{Key: "x", Names: names(nil), IconURL: ptr.To("http://ci.example/i.png")}, InvalidIcon},
		{"icon relative", CreateKind{Key: "x", Names: names(nil), IconURL: ptr.To("/i.png")}, InvalidIcon},
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
	if err != nil || c.Names != nil || c.Icon != nil || c.Position != nil || c.IconURLSet {
		t.Fatalf("%+v %v", c, err)
	}
	if c, err = ValidateKindChanges(UpdateKind{Icon: ptr.To("book-text"), Names: names(nil)}); err != nil || *c.Icon != "book-text" || c.Names["ru"] != "Логи" {
		t.Fatalf("%+v %v", c, err)
	}
	if c, err = ValidateKindChanges(UpdateKind{IconURLSet: true}); err != nil || !c.IconURLSet || c.IconURL != nil {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := ValidateKindChanges(UpdateKind{Names: map[string]*string{}}); err != InvalidKindNames {
		t.Fatal(err)
	}
}

func TestOrderRules(t *testing.T) {
	if got, err := ValidateOrder([]string{"Sentry", "logs"}); err != nil || got[0] != "sentry" || got[1] != "logs" {
		t.Fatalf("%v %v", got, err)
	}
	for _, in := range [][]string{nil, {"logs", "logs"}, {"a_b"}, make([]string, 1001)} {
		if _, err := ValidateOrder(in); err != InvalidKindOrder {
			t.Fatalf("%v: %v", in, err)
		}
	}
}
