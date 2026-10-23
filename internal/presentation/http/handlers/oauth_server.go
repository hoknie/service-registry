package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
)

const OAuthBodyLimit = 64 << 10

func oauthStatus(code string) int {
	switch code {
	case "invalid_client":
		return http.StatusUnauthorized
	case "rate_limited":
		return http.StatusTooManyRequests
	case "server_error":
		return http.StatusInternalServerError
	}
	return http.StatusBadRequest
}

func sendOAuthError(c fiber.Ctx, status int, e *accesstypes.OAuthError) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if e.RetryAfter > 0 {
		c.Set(fiber.HeaderRetryAfter, strconv.FormatUint(e.RetryAfter, 10))
	}
	return c.Status(status).JSON(responses.OAuthFailure{Error: e.Code, Description: e.Description})
}

func oauthFail(c fiber.Ctx, err error) error {
	if e, ok := errors.AsType[*accesstypes.OAuthError](err); ok {
		return sendOAuthError(c, oauthStatus(e.Code), e)
	}
	if e := apperr.From(err); e != nil && e.Kind == apperr.NotFound {
		return responses.Fail(c, err)
	}
	slog.Error("oauth endpoint failed", "err", err)
	return sendOAuthError(c, http.StatusInternalServerError, accesstypes.OAuthFail("server_error", "internal server error"))
}

func OAuthBodyTooLarge(c fiber.Ctx) error {
	return sendOAuthError(c, http.StatusRequestEntityTooLarge, accesstypes.OAuthFail("invalid_request", "the body is larger than 64 KiB"))
}

func (a *Handlers) ProtectedResourceMetadata(c fiber.Ctx) error {
	meta, err := a.Access.AuthorizationServer.Metadata()
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "max-age=3600")
	return c.Status(http.StatusOK).JSON(responses.ProtectedResourceOf(meta))
}

func (a *Handlers) AuthorizationServerMetadata(c fiber.Ctx) error {
	meta, err := a.Access.AuthorizationServer.Metadata()
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "max-age=3600")
	return c.Status(http.StatusOK).JSON(responses.AuthorizationServerOf(meta))
}

type registerBody struct {
	ClientName              *string  `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

func (a *Handlers) RegisterOAuthClient(c fiber.Ctx) error {
	if len(c.Body()) > OAuthBodyLimit {
		return OAuthBodyTooLarge(c)
	}
	var body registerBody
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		if _, err := a.Access.AuthorizationServer.Metadata(); err != nil {
			return responses.Fail(c, err)
		}
		return sendOAuthError(c, http.StatusBadRequest, accesstypes.OAuthFail("invalid_client_metadata", "the body must be a JSON object"))
	}
	client, err := a.Access.AuthorizationServer.Register(c.Context(), c.IP(), accesstypes.ClientMetadata{Name: body.ClientName,
		RedirectURIs: body.RedirectURIs, TokenEndpointAuthMethod: body.TokenEndpointAuthMethod, GrantTypes: body.GrantTypes,
		ResponseTypes: body.ResponseTypes})
	if err != nil {
		return oauthFail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(http.StatusCreated).JSON(responses.RegisteredClientOf(client))
}

func (a *Handlers) OAuthAuthorize(c fiber.Ctx) error {
	location, err := a.Access.AuthorizationServer.StartAuthorization(c.Context(), accesstypes.AuthorizeQuery{
		ResponseType: c.Query("response_type"), ClientID: c.Query("client_id"), RedirectURI: c.Query("redirect_uri"),
		CodeChallenge: c.Query("code_challenge"), CodeChallengeMethod: c.Query("code_challenge_method"),
		Scope: c.Query("scope"), State: c.Query("state"), Resource: c.Query("resource"), PeerIP: c.IP(),
		Superadmin: a.sessionSuperadmin(c),
	})
	if err != nil {
		return oauthFail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Redirect().Status(http.StatusFound).To(location)
}

func (a *Handlers) sessionSuperadmin(c fiber.Ctx) bool {
	token := c.Cookies(SessionCookie)
	if token == "" {
		return false
	}
	p, _, err := a.Access.Auth.Authenticate(c.Context(), token)
	return err == nil && p.IsSuperadmin
}

func (a *Handlers) OAuthToken(c fiber.Ctx) error {
	if len(c.Body()) > OAuthBodyLimit {
		return OAuthBodyTooLarge(c)
	}
	pair, err := a.Access.AuthorizationServer.Token(c.Context(), accesstypes.TokenRequest{
		GrantType: c.FormValue("grant_type"), Code: c.FormValue("code"), RedirectURI: c.FormValue("redirect_uri"),
		ClientID: c.FormValue("client_id"), CodeVerifier: c.FormValue("code_verifier"),
		RefreshToken: c.FormValue("refresh_token"), Resource: c.FormValue("resource"),
	})
	if err != nil {
		return oauthFail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	return c.Status(http.StatusOK).JSON(responses.IssuedTokensOf(pair))
}

func (a *Handlers) OAuthRevoke(c fiber.Ctx) error {
	if len(c.Body()) > OAuthBodyLimit {
		return OAuthBodyTooLarge(c)
	}
	if err := a.Access.AuthorizationServer.Revoke(c.Context(), c.FormValue("client_id"), c.FormValue("token")); err != nil {
		return oauthFail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(http.StatusOK)
}
