package types

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	FailUnreachable       = "gateway.unreachable"
	FailTimeout           = "gateway.timeout"
	FailUnauthorized      = "gateway.unauthorized"
	FailBlocked           = "gateway.blocked"
	FailProxy             = "gateway.proxy"
	FailProtocol          = "gateway.protocol"
	FailSecretUnavailable = "gateway.secret_unavailable"
	FailTooLarge          = "gateway.result_too_large"
)

type Failure struct {
	Code   string
	Detail string
}

func (f *Failure) Error() string {
	if f.Detail == "" {
		return f.Code
	}
	return f.Code + ": " + f.Detail
}

func Fail(code, detail string) error { return &Failure{Code: code, Detail: detail} }

func FailureCode(err error) string {
	var f *Failure
	switch {
	case errors.As(err, &f):
		return f.Code
	case errors.Is(err, context.DeadlineExceeded):
		return FailTimeout
	}
	return FailUnreachable
}

func FailureDetail(err error) string {
	var f *Failure
	if errors.As(err, &f) {
		return f.Detail
	}
	return err.Error()
}

func Redact(text string, secrets ...string) string {
	for _, s := range secrets {
		if len(s) >= 4 {
			text = strings.ReplaceAll(text, s, "***")
		}
	}
	return truncate(strings.TrimSpace(text), 500)
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
