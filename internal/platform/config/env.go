package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
)

type Env map[string]string

func ProcessEnv() Env {
	out := Env{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			out[k] = v
		}
	}
	return out
}

type checker interface{ check() Errors }

type envChecker interface{ checkEnv(Env) Errors }

var validate = newValidate()

func newValidate() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("env"), ",")
		return name
	})
	return v
}

func Load[T any](vars Env) (T, error) {
	var out T
	set := map[string]string{}
	for k, v := range vars {
		if strings.TrimSpace(v) != "" {
			set[k] = v
		}
	}
	var errs Errors
	if err := env.ParseWithOptions(&out, env.Options{Environment: set}); err != nil {
		errs = append(errs, parseErrors(err, envNames(reflect.TypeFor[T](), ""), set)...)
	}
	if err := validate.Struct(&out); err != nil {
		var ve validator.ValidationErrors
		if !errors.As(err, &ve) {
			return out, err
		}
		for _, fe := range ve {
			if !slices.Contains(errs.Vars(), fe.Field()) {
				errs = append(errs, rangeError(fe))
			}
		}
	}
	if c, ok := any(&out).(checker); ok {
		for _, e := range c.check() {
			if !slices.Contains(errs.Vars(), e.Var) {
				errs = append(errs, e)
			}
		}
	}
	if c, ok := any(&out).(envChecker); ok {
		for _, e := range c.checkEnv(set) {
			if !slices.Contains(errs.Vars(), e.Var) {
				errs = append(errs, e)
			}
		}
	}
	if len(errs) > 0 {
		var zero T
		return zero, errs
	}
	return out, nil
}

func envNames(t reflect.Type, prefix string) map[string]string {
	out := map[string]string{}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("env"), ",")
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if name == "" && ft.Kind() == reflect.Struct {
			for k, v := range envNames(ft, prefix+f.Tag.Get("envPrefix")) {
				out[k] = v
			}
			continue
		}
		if name != "" {
			out[f.Name] = prefix + name
		}
	}
	return out
}

func parseErrors(err error, names map[string]string, set map[string]string) Errors {
	var agg env.AggregateError
	list := []error{err}
	if errors.As(err, &agg) {
		list = agg.Errors
	}
	var out Errors
	for _, e := range list {
		var missing env.VarIsNotSetError
		var parse env.ParseError
		switch {
		case errors.As(e, &missing):
			out = append(out, &Error{Var: missing.Key, Reason: "is required"})
		case errors.As(e, &parse):
			name := names[parse.Name]
			reason := fmt.Sprintf("is invalid (%v), got %q", parse.Err, set[name])
			var num *strconv.NumError
			if errors.As(parse.Err, &num) {
				reason = fmt.Sprintf("expected %s, got %q", expected(parse.Type), num.Num)
			}
			out = append(out, &Error{Var: name, Reason: reason})
		default:
			out = append(out, &Error{Var: "environment", Reason: e.Error()})
		}
	}
	return out
}

func expected(t reflect.Type) string {
	if t.Kind() == reflect.Bool {
		return "true or false"
	}
	return "an integer"
}

func rangeError(fe validator.FieldError) *Error {
	switch fe.Tag() {
	case "min":
		return &Error{Var: fe.Field(), Reason: fmt.Sprintf("must be at least %s, got %v", fe.Param(), fe.Value())}
	case "max":
		return &Error{Var: fe.Field(), Reason: fmt.Sprintf("must be at most %s, got %v", fe.Param(), fe.Value())}
	}
	return &Error{Var: fe.Field(), Reason: fmt.Sprintf("fails %s %s", fe.Tag(), fe.Param())}
}
