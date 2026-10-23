package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/google/uuid"
)

const MaxBodyBytes = 64 << 10

type NodeRef struct {
	ID   uuid.UUID `json:"id"`
	Kind string    `json:"kind"`
	Name string    `json:"name"`
	Path string    `json:"path"`
	URL  *string   `json:"url"`
}

type Contact struct {
	Kind  string  `json:"kind"`
	Value string  `json:"value"`
	Label *string `json:"label"`
}

type Owner struct {
	SubjectKind string    `json:"subject_kind"`
	SubjectID   uuid.UUID `json:"subject_id"`
	Name        string    `json:"name"`
	Email       *string   `json:"email"`
	Status      *string   `json:"status"`
	Contacts    []Contact `json:"contacts"`
	OnCallURL   *string   `json:"on_call_url"`
}

type Payload struct {
	ID         uuid.UUID       `json:"id"`
	Type       EventType       `json:"type"`
	OccurredAt string          `json:"occurred_at"`
	Node       NodeRef         `json:"node"`
	Owners     []Owner         `json:"owners"`
	Data       json.RawMessage `json:"data"`
	Recovers   *uuid.UUID      `json:"recovers,omitempty"`
}

func NodeURL(publicURL string, locale string, id uuid.UUID) *string {
	if publicURL == "" {
		return nil
	}
	u := strings.TrimSuffix(publicURL, "/") + "/" + locale + "/catalog?node=" + id.String()
	return &u
}

func (p Payload) ForLocale(locale string) Payload {
	if p.Node.URL != nil {
		u := strings.Replace(*p.Node.URL, "/"+DefaultLocale+"/catalog?node=", "/"+locale+"/catalog?node=", 1)
		p.Node.URL = &u
	}
	return p
}

func EncodePayload(p Payload) ([]byte, error) {
	if p.Owners == nil {
		p.Owners = []Owner{}
	}
	body, err := json.Marshal(p)
	if err != nil || len(body) <= MaxBodyBytes {
		return body, err
	}
	p.Owners = []Owner{}
	return json.Marshal(p)
}

func DriftKey(projectID uuid.UUID, service, environment string, deploymentID uuid.UUID, observed []ObservedVersion) string {
	versions := make([]string, 0, len(observed))
	for _, o := range observed {
		versions = append(versions, o.Version)
	}
	slices.Sort(versions)
	versions = slices.Compact(versions)
	sum := sha256.Sum256([]byte(strings.Join([]string{projectID.String(), service, environment, deploymentID.String(),
		strings.Join(versions, "\x00")}, "\x01")))
	return "drift:" + hex.EncodeToString(sum[:])
}
