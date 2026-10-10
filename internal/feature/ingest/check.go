package ingest

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var check = newCheck()

var (
	dottedRe       = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$`)
	dnsLabelRe     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	visibleASCIIRe = regexp.MustCompile(`^[!-~]+$`)
)

func newCheck() *validator.Validate {
	v := validator.New()
	str := func(ok func(string) bool) validator.Func {
		return func(fl validator.FieldLevel) bool { return ok(fl.Field().String()) }
	}
	must(v.RegisterValidation("dotted", str(dottedRe.MatchString)))
	must(v.RegisterValidation("dnslabel", str(dnsLabelRe.MatchString)))
	must(v.RegisterValidation("visibleascii", str(visibleASCIIRe.MatchString)))
	must(v.RegisterValidation("token", str(func(s string) bool {
		return strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
	})))
	must(v.RegisterValidation("nocontrol", str(func(s string) bool { return strings.IndexFunc(s, unicode.IsControl) < 0 })))
	return v
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func valid(s, tags string) bool { return check.Var(s, tags) == nil }

func trim(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

type fields []FieldError

func (f *fields) add(path string, code FieldCode) {
	*f = append(*f, FieldError{Path: path, Code: code})
}

type rule struct {
	norm func(string) string
	tags string
}

func lowerTrim(s string) string { return strings.ToLower(trim(s)) }

func (f *fields) required(path string, v Field[string], r rule) string {
	if missing(v) {
		f.add(path, FieldRequired)
		return ""
	}
	s, _ := f.value(path, v, r)
	return s
}

func missing(v Field[string]) bool {
	return v.State == Absent || v.State == Null || (v.State == Set && trim(v.Value) == "")
}

func (f *fields) optional(path string, v Field[string], r rule) *string {
	if s, ok := f.value(path, v, r); ok {
		return &s
	}
	return nil
}

func (f *fields) value(path string, v Field[string], r rule) (string, bool) {
	switch {
	case v.State == WrongType:
		f.add(path, FieldInvalidType)
		return "", false
	case missing(v):
		return "", false
	}
	s := v.Value
	if r.norm != nil {
		s = r.norm(s)
	}
	if !valid(s, r.tags) {
		f.add(path, FieldInvalidValue)
		return "", false
	}
	return s, true
}

func Name(raw string) (string, bool) {
	s := lowerTrim(raw)
	return s, dottedRe.MatchString(s)
}
