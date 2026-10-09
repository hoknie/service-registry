package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/apperr"
)

type APIError struct {
	Status     int
	Code       string
	Message    string
	RetryAfter uint64
	Challenge  string
	Fields     []apperr.Field
}

type errorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Fields  []fieldBody `json:"fields,omitempty"`
}

type fieldBody struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func FromApp(e *apperr.Error) *APIError {
	msg := e.Error()
	switch e.Kind {
	case apperr.NotFound:
		return NewAPIError(http.StatusNotFound, "not_found", "not found")
	case apperr.Unavailable:
		if e.Code != "" {
			return NewAPIError(http.StatusServiceUnavailable, e.Code, e.What+" is unavailable")
		}
		return NewAPIError(http.StatusServiceUnavailable, "service_unavailable", e.What+" is unavailable")
	case apperr.Unauthenticated:
		return NewAPIError(http.StatusUnauthorized, "auth.unauthenticated", msg)
	case apperr.InvalidCredentials:
		return NewAPIError(http.StatusUnauthorized, "auth.invalid_credentials", msg)
	case apperr.Forbidden:
		return NewAPIError(http.StatusForbidden, "auth.forbidden", msg)
	case apperr.AccountDisabled:
		return NewAPIError(http.StatusForbidden, "auth.account_disabled", msg)
	case apperr.WrongPassword:
		return NewAPIError(http.StatusForbidden, "auth.wrong_password", msg)
	case apperr.RateLimited:
		return &APIError{Status: http.StatusTooManyRequests, Code: "auth.rate_limited", Message: msg, RetryAfter: e.RetryAfterSecs}
	case apperr.Validation:
		return NewAPIError(http.StatusBadRequest, e.Code, e.Message)
	case apperr.Conflict:
		return NewAPIError(http.StatusConflict, e.Code, e.Message)
	case apperr.InvalidProjectKey:
		return &APIError{Status: http.StatusUnauthorized, Code: "auth.invalid_project_key", Message: msg, Challenge: "Bearer"}
	case apperr.IngestRateLimited:
		return &APIError{Status: http.StatusTooManyRequests, Code: "ingest.rate_limited", Message: msg, RetryAfter: e.RetryAfterSecs}
	case apperr.InvalidToken:
		return &APIError{Status: http.StatusUnauthorized, Code: "auth.invalid_token", Message: msg, Challenge: "Bearer"}
	case apperr.InsufficientScope:
		return NewAPIError(http.StatusForbidden, "auth.insufficient_scope", msg)
	case apperr.SessionRequired:
		return NewAPIError(http.StatusForbidden, "auth.session_required", msg)
	case apperr.UpstreamRateLimited:
		return &APIError{Status: http.StatusTooManyRequests, Code: e.Code, Message: e.Message, RetryAfter: e.RetryAfterSecs}
	case apperr.BadGateway:
		return NewAPIError(http.StatusBadGateway, e.Code, e.Message)
	case apperr.InvalidWebhookSignature:
		return NewAPIError(http.StatusUnauthorized, "auth.invalid_webhook_signature", msg)
	case apperr.Unprocessable:
		return &APIError{Status: http.StatusUnprocessableEntity, Code: e.Code, Message: e.Message, Fields: e.Fields}
	}
	slog.Error("internal error", "detail", e.Detail)
	return NewAPIError(http.StatusInternalServerError, "internal", "internal server error")
}

func (e *APIError) Send(c fiber.Ctx) error {
	if e.RetryAfter > 0 {
		c.Set(fiber.HeaderRetryAfter, strconv.FormatUint(e.RetryAfter, 10))
	}
	if e.Challenge != "" {
		c.Set(fiber.HeaderWWWAuthenticate, e.Challenge)
	}
	body := errorBody{Code: e.Code, Message: e.Message}
	for _, f := range e.Fields {
		body.Fields = append(body.Fields, fieldBody{Path: f.Path, Code: f.Code})
	}
	return c.Status(e.Status).JSON(body)
}

func Fail(c fiber.Ctx, err error) error {
	if api, ok := err.(*APIError); ok {
		return api.Send(c)
	}
	return FromApp(apperr.From(err)).Send(c)
}
