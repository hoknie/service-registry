package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"svc-registry/internal/platform/apperr"
)

const truncated = "\n\n[truncated: the result is larger than the limit of this server; " +
	"request a file by its path, a narrower path or the next page]"

func text(s string, limit int) *sdk.CallToolResult {
	if len(s) > limit {
		cut := limit - len(truncated)
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:max(cut, 0)] + truncated
	}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: s}}}
}

func jsonResult(v any, limit int) (*sdk.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return text(string(b), limit), nil, nil
}

func failed(err error) (*sdk.CallToolResult, any, error) {
	e := apperr.From(err)
	code := "internal"
	switch {
	case e == nil:
	case e.Kind == apperr.NotFound:
		code = "not_found"
	case e.Kind == apperr.Forbidden:
		code = "auth.forbidden"
	case e.Code != "":
		code = e.Code
	}
	msg := code
	var app *apperr.Error
	if errors.As(err, &app) && app.Message != "" && code != "internal" && code != "not_found" {
		msg = fmt.Sprintf("%s: %s", code, app.Message)
	}
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: msg}}}, nil, nil
}
