package http

import (
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	deploytypes "svc-registry/internal/feature/deploy/types"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/presentation/http/handlers"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

const BodyLimit = 2 << 20

func newApp(api *handlers.Handlers) *fiber.App {
	app := fiber.New(fiber.Config{
		StrictRouting:             true,
		CaseSensitive:             true,
		Immutable:                 true,
		DisableDefaultContentType: true,
		BodyLimit:                 BodyLimit,
		StructValidator:           handlers.NewStructValidator(),
		ErrorHandler:              func(c fiber.Ctx, err error) error { return errorHandler(api, c, err) },
	})
	var skillsLimit int64
	if api.Skills != nil && api.Config != nil {
		skillsLimit = api.Config.Skills.MaxUploadBytes
	}
	app.Server().HeaderReceived = func(h *fasthttp.RequestHeader) fasthttp.RequestConfig { return routeBodyLimit(h, skillsLimit) }
	return app
}

func routeBodyLimit(header *fasthttp.RequestHeader, skillsLimit int64) fasthttp.RequestConfig {
	path, _, _ := strings.Cut(string(header.RequestURI()), "?")
	if _, ok := agentReport(string(header.Method()), path); ok {
		return fasthttp.RequestConfig{MaxRequestBodySize: deploytypes.ReportWireLimit}
	}
	if skillsLimit > 0 && skillUpload(string(header.Method()), path) {
		return fasthttp.RequestConfig{MaxRequestBodySize: int(skillsLimit) + requests.UploadSlack}
	}
	return fasthttp.RequestConfig{}
}

func errorHandler(api *handlers.Handlers, c fiber.Ctx, err error) error {
	path := string(c.Request().URI().PathOriginal())
	isAPI := path == "/api" || strings.HasPrefix(path, "/api/")
	status := nethttp.StatusInternalServerError
	fe, isFiber := errors.AsType[*fiber.Error](err)
	if isFiber {
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
		if segment, ok := agentReport(c.Method(), path); ok {
			return api.AgentReportTooLarge(c, segment)
		}
		if strings.HasPrefix(path, "/api/oauth/") {
			return handlers.OAuthBodyTooLarge(c)
		}
		if skillUpload(c.Method(), path) {
			return responses.Fail(c, skillstypes.UploadTooLarge)
		}
		return handlers.InvalidBody("length limit exceeded").Send(c)
	case nethttp.StatusNotFound:
		return handlers.APINotFound(c)
	case nethttp.StatusMethodNotAllowed:
		return handlers.APIMethodNotAllowed(c)
	}
	if isFiber && status < 500 {
		return responses.NewAPIError(status, "validation.invalid_body", fe.Message).Send(c)
	}
	return responses.Fail(c, err)
}

func agentReport(method, path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/v1/ingest/clusters/")
	if !ok || method != fiber.MethodPost {
		return "", false
	}
	segment, ok := strings.CutSuffix(rest, "/reports")
	return segment, ok && segment != "" && !strings.Contains(segment, "/")
}

func ingestProject(method, path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/api/v1/ingest/projects/")
	if !ok || method != fiber.MethodPost {
		return "", false
	}
	segment, ok := strings.CutSuffix(rest, "/events")
	return segment, ok && segment != "" && !strings.Contains(segment, "/")
}
