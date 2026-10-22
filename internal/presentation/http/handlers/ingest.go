package handlers

import (
	"bytes"
	"mime"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/google/uuid"

	ingestservices "svc-registry/internal/feature/ingest/services"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
)

var bearerKey = extractors.FromAuthHeader("Bearer")

func payloadTooLarge() *responses.APIError {
	return responses.NewAPIError(http.StatusRequestEntityTooLarge, "ingest.payload_too_large",
		"the body must be at most "+strconv.Itoa(ingesttypes.BodyLimit)+" bytes")
}

func (a *Handlers) authenticateIngest(c fiber.Ctx, rawProjectID string) (ingestservices.Authenticated, error) {
	projectID, err := uuid.Parse(rawProjectID)
	if err != nil {
		return ingestservices.Authenticated{}, apperr.New(apperr.NotFound)
	}
	var key *string
	if k, err := bearerKey.Extract(c); err == nil {
		key = &k
	}
	return a.Ingest.Accept.AuthenticateIngest(c.Context(), projectID, key)
}

func (a *Handlers) IngestEvent(c fiber.Ctx) error {
	who, err := a.authenticateIngest(c, c.Params("project_id"))
	if err != nil {
		return responses.Fail(c, err)
	}
	raw := c.Body()
	if len(raw) > ingesttypes.BodyLimit {
		return payloadTooLarge().Send(c)
	}
	if mt, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType)); err != nil || mt != fiber.MIMEApplicationJSON {
		return InvalidBody("Content-Type must be application/json").Send(c)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return InvalidBody("the body must be a JSON object").Send(c)
	}
	var envelope ingesttypes.WireEnvelope
	if err := c.Bind().JSON(&envelope); err != nil {
		return InvalidBody(describe(err)).Send(c)
	}
	accepted, err := a.Ingest.Accept.AcceptEvent(c.Context(), who, envelope)
	if err != nil {
		return responses.Fail(c, err)
	}
	status := http.StatusCreated
	if accepted.Replayed {
		status = http.StatusOK
	}
	return c.Status(status).JSON(responses.AcceptedOf(accepted))
}

func (a *Handlers) IngestBodyTooLarge(c fiber.Ctx, projectSegment string) error {
	if _, err := a.authenticateIngest(c, projectSegment); err != nil {
		return responses.Fail(c, err)
	}
	return payloadTooLarge().Send(c)
}
