package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"svc-registry/internal/platform/config"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) MetricsPage() fiber.Handler {
	page := adaptor.HTTPHandler(a.Metrics.Handler())
	token := []byte(a.Config.Metrics.Token)
	return func(c fiber.Ctx) error {
		if len(token) > 0 {
			got, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), token) != 1 {
				return (&responses.APIError{Status: http.StatusUnauthorized, Code: "auth.invalid_metrics_token",
					Message: "a valid metrics token is required", Challenge: "Bearer"}).Send(c)
			}
		}
		if err := page(c); err != nil {
			return err
		}
		c.Set(fiber.HeaderCacheControl, "no-store")
		return nil
	}
}

func (a *Handlers) CountIngest(h fiber.Handler) fiber.Handler {
	if a.Metrics == nil {
		return h
	}
	return func(c fiber.Ctx) error {
		err := h(c)
		a.recordIngest(c)
		return err
	}
}

func (a *Handlers) recordIngest(c fiber.Ctx) {
	if a.Metrics == nil {
		return
	}
	var envelope struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(c.Body(), &envelope)
	outcome := "accepted"
	switch status := c.Response().StatusCode(); {
	case status >= http.StatusBadRequest:
		outcome = "rejected"
	case status == http.StatusOK:
		outcome = "replayed"
	}
	a.Metrics.IngestEvent(envelope.Type, outcome, errorCodeOf(c))
}

func (a *Handlers) CountAgentReport(h fiber.Handler) fiber.Handler {
	if a.Metrics == nil {
		return h
	}
	return func(c fiber.Ctx) error {
		err := h(c)
		a.Metrics.AgentReport(c.Response().StatusCode() < http.StatusBadRequest)
		return err
	}
}

func (a *Handlers) CountForgeHook(h fiber.Handler) fiber.Handler {
	if a.Metrics == nil {
		return h
	}
	return func(c fiber.Ctx) error {
		err := h(c)
		a.Metrics.ForgeWebhook(c.Response().StatusCode() < http.StatusBadRequest)
		return err
	}
}

func (a *Handlers) CountLogin(h fiber.Handler) fiber.Handler {
	if a.Metrics == nil {
		return h
	}
	return func(c fiber.Ctx) error {
		err := h(c)
		switch status := c.Response().StatusCode(); {
		case status < http.StatusBadRequest:
			a.Metrics.Login("password", "succeeded")
		case status == http.StatusTooManyRequests:
			a.Metrics.Login("password", "throttled")
		case status < http.StatusInternalServerError:
			a.Metrics.Login("password", "failed")
		}
		return err
	}
}

func (a *Handlers) CountOAuthLogin(h fiber.Handler) fiber.Handler {
	if a.Metrics == nil {
		return h
	}
	return func(c fiber.Ctx) error {
		provider := c.Params("provider")
		err := h(c)
		known := slices.ContainsFunc(a.Config.OAuth.Providers, func(p config.OAuthProvider) bool { return p.Key == provider })
		if !known || c.Response().StatusCode() >= http.StatusInternalServerError {
			return err
		}
		outcome := "succeeded"
		if strings.Contains(string(c.Response().Header.Peek(fiber.HeaderLocation)), "error=") {
			outcome = "failed"
		}
		a.Metrics.Login("oauth:"+provider, outcome)
		return err
	}
}

func errorCodeOf(c fiber.Ctx) string {
	if c.Response().StatusCode() < http.StatusBadRequest {
		return ""
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(c.Response().Body(), &body)
	return body.Code
}
