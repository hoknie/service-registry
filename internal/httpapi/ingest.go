package httpapi

import (
	"bytes"
	"mime"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/ingest"
	"svc-registry/internal/service"
)

var bearerKey = extractors.FromAuthHeader("Bearer")

func payloadTooLarge() *APIError {
	return NewAPIError(http.StatusRequestEntityTooLarge, "ingest.payload_too_large",
		"the body must be at most "+strconv.Itoa(ingest.BodyLimit)+" bytes")
}

func (a *API) authenticateIngest(c fiber.Ctx, rawProjectID string) (service.Authenticated, error) {
	projectID, err := uuid.Parse(rawProjectID)
	if err != nil {
		return service.Authenticated{}, apperr.New(apperr.NotFound)
	}
	var key *string
	if k, err := bearerKey.Extract(c); err == nil {
		key = &k
	}
	return service.AuthenticateIngest(c.Context(), a.State, projectID, key)
}

func (a *API) Ingest(c fiber.Ctx) error {
	who, err := a.authenticateIngest(c, c.Params("project_id"))
	if err != nil {
		return Fail(c, err)
	}
	raw := c.Body()
	if len(raw) > ingest.BodyLimit {
		return payloadTooLarge().Send(c)
	}
	if mt, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType)); err != nil || mt != fiber.MIMEApplicationJSON {
		return InvalidBody("Content-Type must be application/json").Send(c)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return InvalidBody("the body must be a JSON object").Send(c)
	}
	var envelope ingest.WireEnvelope
	if err := c.Bind().JSON(&envelope); err != nil {
		return InvalidBody(describe(err)).Send(c)
	}
	accepted, err := service.AcceptEvent(c.Context(), a.State, who, envelope)
	if err != nil {
		return Fail(c, err)
	}
	status := http.StatusCreated
	if accepted.Replayed {
		status = http.StatusOK
	}
	return c.Status(status).JSON(response.AcceptedOf(accepted))
}

func (a *API) IngestBodyTooLarge(c fiber.Ctx, projectSegment string) error {
	if _, err := a.authenticateIngest(c, projectSegment); err != nil {
		return Fail(c, err)
	}
	return payloadTooLarge().Send(c)
}
