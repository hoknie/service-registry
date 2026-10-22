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
	var f *deploytypes.Failure
	if errors.As(err, &f) {
		return f
	}
	code := deploytypes.FailUnreachable
	var netErr net.Error
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verify *tls.CertificateVerificationError
	switch {
	case apierrors.IsUnauthorized(err):
		code = deploytypes.FailUnauthorized
	case apierrors.IsForbidden(err):
		code = deploytypes.FailForbidden
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), errors.Is(err, context.DeadlineExceeded),
		errors.As(err, &netErr) && netErr.Timeout():
		code = deploytypes.FailTimeout
	case errors.As(err, &unknownAuthority), errors.As(err, &hostname), errors.As(err, &invalid), errors.As(err, &verify),
		strings.Contains(err.Error(), "x509:"), strings.Contains(err.Error(), "tls:"):
		code = deploytypes.FailTLS
	}
	return &deploytypes.Failure{Code: code, Message: err.Error()}
}
