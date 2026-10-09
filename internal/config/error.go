package config

import "strings"

type Error struct {
	Var    string
	Reason string
}

func (e *Error) Error() string { return e.Var + " " + e.Reason }

type Errors []*Error

func (e Errors) Error() string {
	parts := make([]string, len(e))
	for i, one := range e {
		parts[i] = one.Error()
	}
	return strings.Join(parts, "; ")
}

func (e Errors) Vars() []string {
	out := make([]string, len(e))
	for i, one := range e {
		out[i] = one.Var
	}
	return out
}
