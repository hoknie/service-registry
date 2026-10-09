package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/httpapi/mcp"
	"svc-registry/internal/service"
)

const MCPBodyLimit = 1 << 20

var MCPVersion = "dev"

func (a *API) MCP(c fiber.Ctx) error {
	if !sameHostOrigin(c) {
		return NewAPIError(http.StatusForbidden, "auth.csrf_rejected", "cross-origin MCP request rejected").Send(c)
	}
	header, ok := authorization(c)
	if !ok {
		return (&APIError{Status: http.StatusUnauthorized, Code: "auth.unauthenticated", Message: "a bearer token is required",
			Challenge: "Bearer"}).Send(c)
	}
	secret, isBearer := bearer(header)
	if !isBearer {
		return invalidMCPToken(c)
	}
	p, _, err := service.AuthenticateToken(c.Context(), a.State, secret)
	if err != nil {
		if e := apperr.From(err); e != nil && e.Kind == apperr.InvalidToken {
			return invalidMCPToken(c)
		}
		return Fail(c, err)
	}
	if !p.Scopes.Has(access.ScopeMCP) {
		return Fail(c, apperr.New(apperr.InsufficientScope))
	}
	if len(c.Body()) > MCPBodyLimit {
		return NewAPIError(http.StatusRequestEntityTooLarge, "validation.invalid_body", "the MCP message is larger than 1 MiB").Send(c)
	}
	if refusal := mcpPrecheck(c.Body()); refusal != nil {
		return c.Status(http.StatusOK).JSON(refusal)
	}
	a.mcpOnce.Do(func() { a.mcpHandler = adaptor.HTTPHandlerWithContext(mcp.Handler(a.State, MCPVersion)) })
	c.SetContext(mcp.WithPrincipal(c.Context(), p))
	return a.mcpHandler(c)
}

func invalidMCPToken(c fiber.Ctx) error {
	return (&APIError{Status: http.StatusUnauthorized, Code: "auth.invalid_token", Message: "the bearer token is not valid",
		Challenge: "Bearer"}).Send(c)
}

func sameHostOrigin(c fiber.Ctx) bool {
	origins := c.Request().Header.PeekAll(fiber.HeaderOrigin)
	if len(origins) == 0 {
		return true
	}
	origin := string(origins[0])
	authority, ok := strings.CutPrefix(origin, "https://")
	if !ok {
		authority, ok = strings.CutPrefix(origin, "http://")
	}
	host := string(c.Request().Header.Host())
	return ok && host != "" && strings.EqualFold(authority, host)
}

var mcpMethods = map[string]bool{
	"initialize": true, "ping": true, "tools/list": true, "tools/call": true,
	"resources/list": true, "resources/templates/list": true, "resources/read": true,
}

type mcpError struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func mcpPrecheck(body []byte) *mcpError {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method *string         `json:"method"`
	}
	refuse := func(id json.RawMessage, code int, message string) *mcpError {
		e := &mcpError{JSONRPC: "2.0", ID: id}
		if len(e.ID) == 0 {
			e.ID = json.RawMessage("null")
		}
		e.Error.Code, e.Error.Message = code, message
		return e
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '[' {
			return nil
		}
		return refuse(nil, -32700, "parse error")
	}
	isRequest := len(msg.ID) > 0 && string(msg.ID) != "null"
	if msg.Method != nil && isRequest && !mcpMethods[*msg.Method] {
		return refuse(msg.ID, -32601, "method not found: "+*msg.Method)
	}
	return nil
}
