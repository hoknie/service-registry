package types

import (
	"slices"

	"github.com/google/uuid"
)

type Status string

const (
	StatusOK           Status = "ok"
	StatusRedirect     Status = "redirect"
	StatusAuthRequired Status = "auth_required"
	StatusNotFound     Status = "not_found"
	StatusServerError  Status = "server_error"
	StatusUnexpected   Status = "unexpected"
	StatusTimeout      Status = "timeout"
	StatusTLSError     Status = "tls_error"
	StatusDNSError     Status = "dns_error"
	StatusUnreachable  Status = "unreachable"
	StatusBlocked      Status = "blocked"
)

type Result struct {
	Status     Status
	HTTPStatus *int
	DurationMS int
}

type Recorded struct {
	Check    Check
	TargetID uuid.UUID
	CheckID  uuid.UUID
	Previous *Status
}

type TargetPair struct {
	URL     string
	LinkKey string
}

type Sighting struct {
	URLs  []string
	Pairs []TargetPair
}

func Sightings(links []Link) Sighting {
	out := Sighting{URLs: URLs(links)}
	for _, l := range links {
		if l.URL == nil {
			continue
		}
		if p := (TargetPair{URL: *l.URL, LinkKey: l.LinkKey}); !slices.Contains(out.Pairs, p) {
			out.Pairs = append(out.Pairs, p)
		}
	}
	slices.Sort(out.URLs)
	return out
}

type TargetProject struct {
	ProjectID uuid.UUID
	LinkKey   string
}

type Check struct {
	URL        string
	Status     Status
	HTTPStatus *int
	DurationMS int
	CheckedAt  string
}

func StatusOf(code int) Status {
	switch {
	case code >= 200 && code < 300:
		return StatusOK
	case code == 401 || code == 403:
		return StatusAuthRequired
	case code == 404 || code == 410:
		return StatusNotFound
	case code >= 500 && code < 600:
		return StatusServerError
	}
	return StatusUnexpected
}
