package metrics

import (
	"regexp"
	"strconv"
)

var methods = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true}

func Method(m string) string {
	if methods[m] {
		return m
	}
	return "other"
}

func ResultClass(status int, err error) string {
	if err != nil || status < 100 || status > 599 {
		return "error"
	}
	return strconv.Itoa(status/100) + "xx"
}

var codePattern = regexp.MustCompile(`^[a-z_]+(\.[a-z_]+)+$`)

func Code(c string) string {
	if c == "" || len(c) <= 64 && codePattern.MatchString(c) {
		return c
	}
	return "other"
}

func (m *Metrics) eventType(t string) string {
	if m.eventTypes[t] {
		return t
	}
	return "unknown"
}

func oneOf(v string, allowed ...string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return "other"
}
