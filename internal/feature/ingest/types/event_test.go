package types

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCheckOccurredAtRefusesTheFarFuture(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	cases := map[string]bool{
		"2026-10-24T12:04:59Z":      false,
		"2026-10-24T12:05:00Z":      false,
		"2026-10-24T12:05:01Z":      true,
		"2026-10-24T15:05:01+03:00": true,
		"2020-01-01T00:00:00Z":      false,
		"not a time":                false,
	}
	for at, refused := range cases {
		err := CheckOccurredAt(at, now)
		var r *Rejection
		if got := errors.As(err, &r); got != refused {
			t.Errorf("%s: refused %v, want %v (%v)", at, got, refused, err)
		}
		if refused && (len(r.Fields) != 1 || r.Fields[0].Code != FieldInFuture) {
			t.Errorf("%s: fields %v", at, r.Fields)
		}
	}
}

func TestDeploymentRecordOfAnEvent(t *testing.T) {
	t.Parallel()
	e := NewEvent{ID: uuid.New(), ProjectID: uuid.New(), DeploymentID: uuid.New(), EnvironmentID: uuid.New(),
		Envelope: Envelope{OccurredAt: "2026-10-24T12:00:00Z"}}
	d := e.Deployment(&ServiceDeployed{Service: "api", Environment: "prod", Version: "1.2.3"}).Stored()
	if d.ID != e.DeploymentID || d.EventID == nil || *d.EventID != e.ID || d.Source != "event" || d.OccurredAt != e.Envelope.OccurredAt {
		t.Fatalf("record %+v", d)
	}
	if d.Metadata == nil || d.Links == nil {
		t.Fatal("stored record keeps nil metadata or links")
	}
	if r := d.Result(true); r.ID != d.ID || r.Service != "api" || !r.BecameCurrent {
		t.Fatalf("result %+v", r)
	}
}
