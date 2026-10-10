package k8s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"svc-registry/internal/feature/deploy"
)

func failure(err error) error {
	if err == nil {
		return nil
	}
	var f *deploy.Failure
	if errors.As(err, &f) {
		return f
	}
	code := deploy.FailUnreachable
	var netErr net.Error
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verify *tls.CertificateVerificationError
	switch {
	case apierrors.IsUnauthorized(err):
		code = deploy.FailUnauthorized
	case apierrors.IsForbidden(err):
		code = deploy.FailForbidden
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), errors.Is(err, context.DeadlineExceeded),
		errors.As(err, &netErr) && netErr.Timeout():
		code = deploy.FailTimeout
	case errors.As(err, &unknownAuthority), errors.As(err, &hostname), errors.As(err, &invalid), errors.As(err, &verify),
		strings.Contains(err.Error(), "x509:"), strings.Contains(err.Error(), "tls:"):
		code = deploy.FailTLS
	}
	return &deploy.Failure{Code: code, Message: err.Error()}
}
