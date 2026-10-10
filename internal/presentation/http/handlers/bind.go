package handlers

import (
	"errors"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/presentation/http/responses"
)

type StructValidator struct{ v *validator.Validate }

func NewStructValidator() *StructValidator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		for _, tag := range []string{"json", "query"} {
			if name, _, _ := strings.Cut(f.Tag.Get(tag), ","); name != "" && name != "-" {
				return name
			}
		}
		return f.Name
	})
	return &StructValidator{v: v}
}

func (s *StructValidator) Validate(out any) error { return s.v.Struct(out) }

func InvalidBody(message string) *responses.APIError {
	return responses.NewAPIError(http.StatusBadRequest, "validation.invalid_body", message)
}

func isJSON(c fiber.Ctx) bool {
	mt, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	return err == nil && (mt == fiber.MIMEApplicationJSON || (strings.HasPrefix(mt, "application/") && strings.HasSuffix(mt, "+json")))
}

func bindJSON(c fiber.Ctx, out any) error {
	if !isJSON(c) {
		return InvalidBody("expected Content-Type: application/json")
	}
	if err := c.Bind().JSON(out); err != nil {
		return InvalidBody(describe(err))
	}
	return nil
}

func bindQuery(c fiber.Ctx, out any, fallback error, byField map[string]error) error {
	err := c.Bind().Query(out)
	if err == nil {
		return nil
	}
	var be *fiber.BindError
	if errors.As(err, &be) {
		for name, mapped := range byField {
			if strings.EqualFold(be.Field, name) {
				return mapped
			}
		}
	}
	return fallback
}

func describe(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		parts := make([]string, 0, len(ve))
		for _, fe := range ve {
			if fe.Tag() == "required" {
				parts = append(parts, fe.Field()+" is required")
			} else {
				parts = append(parts, fe.Field()+" fails "+fe.Tag())
			}
		}
		return strings.Join(parts, "; ")
	}
	var be *fiber.BindError
	if errors.As(err, &be) && be.Err != nil {
		return be.Err.Error()
	}
	return err.Error()
}
