package k8s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

func failure(err error) error {
	if err == nil {
		return nil
	}
	if f, ok := errors.AsType[*deploytypes.Failure](err); ok {
		return f
	}
	code := deploytypes.FailUnreachable
	switch {
	case apierrors.IsUnauthorized(err):
		code = deploytypes.FailUnauthorized
	case apierrors.IsForbidden(err):
		code = deploytypes.FailForbidden
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), errors.Is(err, context.DeadlineExceeded),
		timedOut(err):
		code = deploytypes.FailTimeout
	case certificateError(err), strings.Contains(err.Error(), "x509:"), strings.Contains(err.Error(), "tls:"):
		code = deploytypes.FailTLS
	}
	return &deploytypes.Failure{Code: code, Message: err.Error()}
}

func timedOut(err error) bool {
	netErr, ok := errors.AsType[net.Error](err)
	return ok && netErr.Timeout()
}

func certificateError(err error) bool {
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.HostnameError](err); ok {
		return true
	}
	if _, ok := errors.AsType[x509.CertificateInvalidError](err); ok {
		return true
	}
	_, ok := errors.AsType[*tls.CertificateVerificationError](err)
	return ok
}
