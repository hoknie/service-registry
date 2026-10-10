package responses

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/platform/apperr"
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

var statuses = map[apperr.Kind]int{
	apperr.NotFound:                http.StatusNotFound,
	apperr.Unavailable:             http.StatusServiceUnavailable,
	apperr.Unauthenticated:         http.StatusUnauthorized,
	apperr.InvalidCredentials:      http.StatusUnauthorized,
	apperr.Forbidden:               http.StatusForbidden,
	apperr.AccountDisabled:         http.StatusForbidden,
	apperr.WrongPassword:           http.StatusForbidden,
	apperr.RateLimited:             http.StatusTooManyRequests,
	apperr.Validation:              http.StatusBadRequest,
	apperr.Conflict:                http.StatusConflict,
	apperr.InvalidProjectKey:       http.StatusUnauthorized,
	apperr.IngestRateLimited:       http.StatusTooManyRequests,
	apperr.InvalidToken:            http.StatusUnauthorized,
	apperr.InsufficientScope:       http.StatusForbidden,
	apperr.SessionRequired:         http.StatusForbidden,
	apperr.UpstreamRateLimited:     http.StatusTooManyRequests,
	apperr.BadGateway:              http.StatusBadGateway,
	apperr.InvalidWebhookSignature: http.StatusUnauthorized,
	apperr.Unprocessable:           http.StatusUnprocessableEntity,
}

func FromApp(e *apperr.Error) *APIError {
	status, ok := statuses[e.Kind]
	if !ok {
		slog.Error("internal error", "detail", e.Detail)
		return NewAPIError(http.StatusInternalServerError, "internal", "internal server error")
	}
	code, message := e.Public()
	api := &APIError{Status: status, Code: code, Message: message}
	switch e.Kind {
	case apperr.RateLimited, apperr.IngestRateLimited, apperr.UpstreamRateLimited:
		api.RetryAfter = e.RetryAfterSecs
	case apperr.InvalidProjectKey, apperr.InvalidToken:
		api.Challenge = "Bearer"
	case apperr.Unprocessable:
		api.Fields = e.Fields
	}
	return api
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
