package webui

import (
	"reflect"
	"testing"
)

func TestPlainPathsSplitIntoSegments(t *testing.T) {
	for raw, want := range map[string][]string{
		"/":           nil,
		"/ru/catalog": {"ru", "catalog"},
		"/ru/catalog/__next.$d$locale.catalog.__PAGE__.txt": {"ru", "catalog", "__next.$d$locale.catalog.__PAGE__.txt"},
		"/en/a%20b%24":  {"en", "a b$"},
		"/zh/%E4%B8%AD": {"zh", "中"},
	} {
		got, err := parsePath(raw)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %q %v", raw, got, err)
		}
	}
}

func TestTraversalAndSeparatorsAreRefused(t *testing.T) {
	for raw, why := range map[string]error{
		"/en/../etc/passwd": errDot,
		"/en/./x":           errDot,
		"/en/%2e%2e/x":      errDot,
		"/en/%2E%2E/x":      errDot,
		"/en/a%2fb":         errSeparator,
		"/en/a%2Fb":         errSeparator,
		"/en/a%5cb":         errSeparator,
		`/en/a\b`:           errSeparator,
		"/en/a%00b":         errSeparator,
		"/en/a%0ab":         errSeparator,
		"/en//x":            errEmpty,
		"/en/%zz":           errEncoding,
		"/en/%4":            errEncoding,
		"/en/%ff":           errEncoding,
	} {
		if _, err := parsePath(raw); err != why {
			t.Errorf("%s: %v, want %v", raw, err, why)
		}
	}
}
