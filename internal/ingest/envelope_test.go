package ingest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func obj(t *testing.T, text string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return v
}

func base(t *testing.T, edit func(e, p map[string]any)) (Envelope, error) {
	t.Helper()
	e := obj(t, `{"type":"service.deployed","version":1,"occurred_at":"2026-10-07T12:00:00Z",
		"idempotency_key":"run-1","payload":{"service":" API ","version":"1.4.2","environment":"Production"}}`)
	if edit != nil {
		p, _ := e["payload"].(map[string]any)
		edit(e, p)
	}
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var w WireEnvelope
	if err := json.Unmarshal(body, &w); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return ParseEnvelope(w)
}

func set(m map[string]any, key, jsonText string) { m[key] = json.RawMessage(jsonText) }

func fieldList(t *testing.T, err error) []string {
	t.Helper()
	r, ok := err.(*Rejection)
	if !ok || r.Kind != RejectInvalid {
		t.Fatalf("expected invalid fields, got %v", err)
	}
	out := make([]string, 0, len(r.Fields))
	for _, f := range r.Fields {
		out = append(out, f.Path+" "+string(f.Code))
	}
	return out
}

func eqList(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAMinimalEventIsNormalized(t *testing.T) {
	e, err := base(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != TypeServiceDeployed || e.Version != 1 || e.IdempotencyKey != "run-1" {
		t.Errorf("%+v", e)
	}
	want := ServiceDeployed{Service: "api", Version: "1.4.2", Environment: "production", Metadata: map[string]string{}}
	if !reflect.DeepEqual(*e.ServiceDeployed, want) {
		t.Errorf("payload %+v", *e.ServiceDeployed)
	}
	var raw map[string]any
	if err := json.Unmarshal(e.Payload, &raw); err != nil || raw["service"] != " API " {
		t.Error("raw payload is stored as received")
	}
}

func TestTypeIsCheckedFirstThenVersion(t *testing.T) {
	_, err := base(t, func(e, _ map[string]any) { e["type"] = "build.finished"; e["version"] = "x" })
	if r, ok := err.(*Rejection); !ok || r.Kind != RejectUnknownType {
		t.Errorf("unknown type: %v", err)
	}
	_, err = base(t, func(e, _ map[string]any) { delete(e, "type") })
	eqList(t, fieldList(t, err), "type required")
	_, err = base(t, func(e, _ map[string]any) { set(e, "type", "5") })
	eqList(t, fieldList(t, err), "type invalid_type")

	_, err = base(t, func(e, _ map[string]any) { set(e, "version", "2"); set(e, "payload", "[]") })
	if r, ok := err.(*Rejection); !ok || r.Kind != RejectUnsupportedVersion {
		t.Errorf("unsupported version: %v", err)
	}
	for _, v := range []string{"1.5", "1.0", "1e0", "99999999999999999999", `"1"`} {
		_, err = base(t, func(e, _ map[string]any) { set(e, "version", v) })
		eqList(t, fieldList(t, err), "version invalid_type")
	}
	_, err = base(t, func(e, _ map[string]any) { e["version"] = nil })
	eqList(t, fieldList(t, err), "version required")
	_, err = base(t, func(e, _ map[string]any) { set(e, "version", "4294967297") })
	if r, ok := err.(*Rejection); !ok || r.Kind != RejectUnsupportedVersion {
		t.Errorf("large version: %v", err)
	}
}

func TestEveryInvalidFieldIsReportedWithItsPath(t *testing.T) {
	_, err := base(t, func(e, _ map[string]any) {
		delete(e, "idempotency_key")
		set(e, "payload", `{"service":"API!","version":12}`)
	})
	eqList(t, fieldList(t, err),
		"idempotency_key required", "payload.service invalid_value",
		"payload.version invalid_type", "payload.environment required")
}

func TestEnvelopeFieldsHaveRules(t *testing.T) {
	_, err := base(t, func(e, _ map[string]any) {
		e["occurred_at"] = "2026-02-30T10:00:00Z"
		e["idempotency_key"] = "has space"
	})
	eqList(t, fieldList(t, err), "occurred_at invalid_value", "idempotency_key invalid_value")
	_, err = base(t, func(e, _ map[string]any) { e["idempotency_key"] = strings.Repeat("k", 201) })
	eqList(t, fieldList(t, err), "idempotency_key invalid_value")
	_, err = base(t, func(e, _ map[string]any) { set(e, "payload", "[]") })
	eqList(t, fieldList(t, err), "payload invalid_type")
	_, err = base(t, func(e, _ map[string]any) { delete(e, "payload") })
	eqList(t, fieldList(t, err), "payload required")
}

func TestOptionalFieldsAreCheckedAndBlankMeansUnset(t *testing.T) {
	e, err := base(t, func(_, p map[string]any) {
		p["commit_sha"] = "ABCDEF0123"
		p["branch"] = ""
		p["url"] = nil
		p["namespace"] = "prod-1"
		p["cluster"] = " eu-1 "
		p["deployed_by"] = "ci-bot"
		set(p, "metadata", `{"run_id":"42"}`)
		set(p, "extra", `{"ignored":true}`)
	})
	if err != nil {
		t.Fatal(err)
	}
	p := e.ServiceDeployed
	if *p.CommitSHA != "abcdef0123" || p.Branch != nil || p.URL != nil || *p.Namespace != "prod-1" ||
		*p.Cluster != "eu-1" || p.Metadata["run_id"] != "42" {
		t.Errorf("%+v", p)
	}

	_, err = base(t, func(_, p map[string]any) {
		p["commit_sha"] = "xyz"
		p["branch"] = "-x"
		p["namespace"] = "Prod"
		p["url"] = "ftp://example.com"
		set(p, "deployed_by", "7")
	})
	eqList(t, fieldList(t, err),
		"payload.commit_sha invalid_value", "payload.branch invalid_value",
		"payload.namespace invalid_value", "payload.url invalid_value",
		"payload.deployed_by invalid_type")
}

func TestMetadataRules(t *testing.T) {
	_, err := base(t, func(_, p map[string]any) {
		set(p, "metadata", fmt.Sprintf(`{"run_id":42,"Bad Key":"x","long":%q}`, strings.Repeat("v", 1025)))
	})
	got := fieldList(t, err)
	sort.Strings(got)
	eqList(t, got, "payload.metadata.Bad Key invalid_value", "payload.metadata.long invalid_value",
		"payload.metadata.run_id invalid_type")
	_, err = base(t, func(_, p map[string]any) { p["metadata"] = "x" })
	eqList(t, fieldList(t, err), "payload.metadata invalid_type")
	many := map[string]any{}
	for i := range 33 {
		many[fmt.Sprintf("k%d", i)] = "v"
	}
	_, err = base(t, func(_, p map[string]any) { p["metadata"] = many })
	eqList(t, fieldList(t, err), "payload.metadata invalid_value")
}

func TestOccurredAtIsRFC3339WithARealDate(t *testing.T) {
	for _, ok := range []string{"2026-10-07T12:00:00Z", "2026-10-07T12:00:00.5Z", "2026-10-07T12:00:00.123456789+05:30", "2024-02-29T00:00:00Z"} {
		if _, err := base(t, func(e, _ map[string]any) { e["occurred_at"] = ok }); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"2026-10-07", "2026-10-07T12:00:00", "2026-10-07 12:00:00Z", "2026-02-30T10:00:00Z",
		"2025-02-29T00:00:00Z", "2026-10-07T24:00:00Z", "2026-10-07T12:00:00+0530", "+2026-10-07T12:00:00Z"} {
		_, err := base(t, func(e, _ map[string]any) { e["occurred_at"] = bad })
		eqList(t, fieldList(t, err), "occurred_at invalid_value")
	}
}

func TestFieldCodesHaveStableNames(t *testing.T) {
	if FieldInFuture != "in_future" {
		t.Error(FieldInFuture)
	}
	for kind, code := range map[RejectionKind]string{
		RejectUnknownType: "ingest.unknown_event_type", RejectUnsupportedVersion: "ingest.unsupported_version",
		RejectInvalid: "ingest.invalid_event",
	} {
		if got := (&Rejection{Kind: kind}).Code(); got != code {
			t.Errorf("%d: %s", kind, got)
		}
	}
}

func TestDeploymentFilterNormalizes(t *testing.T) {
	s, e, b, blank := " API ", "Production", "feature/x", "  "
	f := NewDeploymentFilter(&s, &e, &b)
	if *f.Service != "api" || *f.Environment != "production" || *f.Branch != "feature/x" {
		t.Errorf("%+v", f)
	}
	f = NewDeploymentFilter(&blank, nil, &blank)
	if f.Service != nil || f.Environment != nil || f.Branch != nil {
		t.Errorf("blank is no filter: %+v", f)
	}
}
