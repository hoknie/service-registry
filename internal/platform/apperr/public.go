package apperr

var fixedCodes = map[Kind]string{ // #nosec G101 -- stable error codes, not credentials
	Unauthenticated:         "auth.unauthenticated",
	InvalidCredentials:      "auth.invalid_credentials",
	Forbidden:               "auth.forbidden",
	AccountDisabled:         "auth.account_disabled",
	WrongPassword:           "auth.wrong_password",
	RateLimited:             "auth.rate_limited",
	InvalidProjectKey:       "auth.invalid_project_key",
	IngestRateLimited:       "ingest.rate_limited",
	InvalidToken:            "auth.invalid_token",
	InsufficientScope:       "auth.insufficient_scope",
	SessionRequired:         "auth.session_required",
	InvalidWebhookSignature: "auth.invalid_webhook_signature",
	InvalidAgentToken:       "auth.invalid_agent_token",
}

func (e *Error) Public() (code, message string) {
	switch e.Kind {
	case NotFound:
		return "not_found", "not found"
	case Unavailable:
		if e.Code != "" {
			return e.Code, e.What + " is unavailable"
		}
		return "service_unavailable", e.What + " is unavailable"
	case Validation, Conflict, UpstreamRateLimited, BadGateway, Unprocessable:
		return e.Code, e.Message
	}
	if c, ok := fixedCodes[e.Kind]; ok {
		return c, e.Error()
	}
	return "internal", "internal server error"
}
