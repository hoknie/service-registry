package types

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func payloadOf(t *testing.T, typ EventType, data any) Payload {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.Must(uuid.NewV7())
	return Payload{ID: id, Type: typ, Node: NodeRef{ID: id, Kind: "project", Name: "API", Path: "acme/backend/api",
		URL: NodeURL("https://registry.example", DefaultLocale, id)}, Data: raw}
}

func slackOf(t *testing.T, p Payload, locale string) slackMessage {
	t.Helper()
	body, err := SlackBody(p.ForLocale(locale), locale)
	if err != nil {
		t.Fatal(err)
	}
	var m slackMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSlackDeploymentInRussian(t *testing.T) {
	t.Parallel()
	p := payloadOf(t, EventDeploymentCreated, DeploymentData{Service: "api", Environment: "production", Version: "2.0.0", Source: "event"})
	m := slackOf(t, p, "ru")
	for _, want := range []string{"Выкат", "acme/backend/api", "2.0.0", "production"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("text %q lacks %q", m.Text, want)
		}
	}
	body, _ := json.Marshal(m)
	if !strings.Contains(string(body), "/ru/catalog?node=") || !strings.Contains(string(body), "Открыть в реестре") {
		t.Errorf("no localized link: %s", body)
	}
	if strings.Contains(string(body), "Владельцы") {
		t.Errorf("owners line without owners: %s", body)
	}
}

func TestSlackEscapesOutsideValues(t *testing.T) {
	t.Parallel()
	p := payloadOf(t, EventDeploymentCreated, DeploymentData{Service: "api", Environment: "production", Version: "<!channel>", Source: "event"})
	p.Owners = []Owner{{Name: "Platform & Ops", OnCallURL: new("https://oncall.example/p")}}
	m := slackOf(t, p, "en")
	if !strings.Contains(m.Text, "&lt;!channel&gt;") || strings.Contains(m.Text, "<!channel>") {
		t.Fatalf("text %q", m.Text)
	}
	body, _ := json.Marshal(m)
	var decoded map[string]any
	_ = json.Unmarshal(body, &decoded)
	if !strings.Contains(jsonString(decoded), "Platform &amp; Ops (<https://oncall.example/p|on call>)") {
		t.Fatalf("owners: %s", jsonString(decoded))
	}
}

func jsonString(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return b.String()
}

func TestEveryLocaleHasTheSameMessages(t *testing.T) {
	t.Parallel()
	en := dictionaries["en"]
	keys := make([]string, 0, len(en))
	for k := range en {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	locales := DictionaryLocales()
	slices.Sort(locales)
	if !slices.Equal(locales, Locales) {
		t.Fatalf("locales %v", locales)
	}
	for _, l := range Locales {
		d := dictionaries[l]
		if len(d) != len(en) {
			t.Errorf("%s has %d messages, en has %d", l, len(d), len(en))
		}
		for _, k := range keys {
			if d[k] == "" {
				t.Errorf("%s lacks %q", l, k)
			}
		}
	}
	for _, e := range append(slices.Clone(SubscribableEvents), EventTest) {
		if en[string(e)] == "" {
			t.Errorf("no title for %s", e)
		}
	}
}

func TestPayloadForLocaleAndSize(t *testing.T) {
	t.Parallel()
	p := payloadOf(t, EventTest, TestData{Subscription: "ops"})
	if got := *p.ForLocale("zh").Node.URL; !strings.Contains(got, "/zh/catalog?node=") {
		t.Fatalf("url %q", got)
	}
	p.Owners = []Owner{{Name: strings.Repeat("x", MaxBodyBytes)}}
	body, err := EncodePayload(p)
	if err != nil || len(body) > MaxBodyBytes || !strings.Contains(string(body), `"owners":[]`) {
		t.Fatalf("%d bytes, %v", len(body), err)
	}
}

func TestSlackRecoveries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		p      Payload
		locale string
		want   []string
	}{
		{"link in spanish", payloadOf(t, EventLinkRecovered, LinkData{LinkKey: "grafana", URL: "https://g.example", Status: "ok",
			PreviousStatus: "server_error"}), "es", []string{"Enlace restablecido", "grafana", "ok"}},
		{"drift in russian", payloadOf(t, EventDriftResolved, DriftResolvedData{Service: "api", Environment: "production",
			Version: "1.5.0"}), "ru", []string{"Расхождение ушло", "1.5.0", "production"}},
		{"hosting in chinese", payloadOf(t, EventRepoSyncRecovered, RepoSyncData{Hosting: "github", OwnerPath: "acme"}), "zh",
			[]string{"仓库同步已恢复", "github acme"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := slackOf(t, tt.p, tt.locale)
			for _, want := range tt.want {
				if !strings.Contains(m.Text, want) {
					t.Errorf("text %q lacks %q", m.Text, want)
				}
			}
		})
	}
}

func TestEpisodeKeysFitTheColumn(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	for _, k := range []string{DriftEpisode(id, strings.Repeat("s", 63), strings.Repeat("e", 63)),
		LinkEpisode(id, id, strings.Repeat("k", 63)), RepoSyncEpisode(id)} {
		if len(k) > 200 {
			t.Errorf("%d bytes: %s", len(k), k)
		}
	}
	if DriftEpisode(id, "a", "b:c") == DriftEpisode(id, "a:b", "c") {
		t.Error("drift episodes collide")
	}
}
