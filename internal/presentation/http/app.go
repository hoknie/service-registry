package http

import (
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/handlers"
	"svc-registry/internal/presentation/http/responses"
)

const BodyLimit = 2 << 20

func newApp(api *handlers.Handlers) *fiber.App {
	return fiber.New(fiber.Config{
		StrictRouting:             true,
		CaseSensitive:             true,
		Immutable:                 true,
		DisableDefaultContentType: true,
		BodyLimit:                 BodyLimit,
		StructValidator:           handlers.NewStructValidator(),
		ErrorHandler:              func(c fiber.Ctx, err error) error { return errorHandler(api, c, err) },
	})
}

func errorHandler(api *handlers.Handlers, c fiber.Ctx, err error) error {
	path := string(c.Request().URI().PathOriginal())
	isAPI := path == "/api" || strings.HasPrefix(path, "/api/")
	var fe *fiber.Error
	status := nethttp.StatusInternalServerError
	if errors.As(err, &fe) {
		status = fe.Code
	}
	if !isAPI {
		c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
		return c.Status(status).SendString(nethttp.StatusText(status) + "\n")
	}
	switch status {
	case nethttp.StatusRequestEntityTooLarge:
		if segment, ok := ingestProject(c.Method(), path); ok {
			return api.IngestBodyTooLarge(c, segment)
		}
		return handlers.InvalidBody("length limit exceeded").Send(c)
	case nethttp.StatusNotFound:
		return handlers.APINotFound(c)
	case nethttp.StatusMethodNotAllowed:
		return handlers.APIMethodNotAllowed(c)
	}
	if fe != nil && status < 500 {
		return responses.NewAPIError(status, "validation.invalid_body", fe.Message).Send(c)
	}
	return responses.Fail(c, err)
}

func ingestProject(method, path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/v1/ingest/projects/")
	if !ok || method != fiber.MethodPost {
		return "", false
	}
	segment, ok := strings.CutSuffix(rest, "/events")
	return segment, ok && segment != "" && !strings.Contains(segment, "/")
}
