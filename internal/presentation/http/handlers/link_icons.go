package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) UploadLinkIcon(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	header, err := c.FormFile("file")
	if err != nil {
		return responses.Fail(c, responses.NewAPIError(http.StatusBadRequest, "validation.invalid_body", "a multipart form with a file field is required"))
	}
	tooLarge := responses.NewAPIError(http.StatusRequestEntityTooLarge, "validation.icon_too_large", "the icon is larger than 64 KiB")
	if header.Size > linkstypes.MaxIconBytes {
		return responses.Fail(c, tooLarge)
	}
	f, err := header.Open()
	if err != nil {
		return responses.Fail(c, responses.NewAPIError(http.StatusBadRequest, "validation.invalid_body", "the file cannot be read"))
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, linkstypes.MaxIconBytes+1))
	if err != nil {
		return responses.Fail(c, responses.NewAPIError(http.StatusBadRequest, "validation.invalid_body", "the file cannot be read"))
	}
	if len(data) > linkstypes.MaxIconBytes {
		return responses.Fail(c, tooLarge)
	}
	icon, err := a.Links.Icons.UploadLinkIcon(c.Context(), p, id, data)
	if errors.Is(err, linkstypes.ErrIconTooLarge) {
		return responses.Fail(c, tooLarge)
	}
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"id": icon, "url": "/api/v1/link-icons/" + icon})
}

func (a *Handlers) LinkIcon(c fiber.Ctx) error {
	if _, _, err := a.currentUser(c); err != nil {
		return responses.Fail(c, err)
	}
	id := c.Params("id")
	f, size, contentType, err := a.Links.Icons.OpenLinkIcon(id)
	if err != nil {
		return responses.Fail(c, err)
	}
	defer f.Close()
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderContentLength, strconv.FormatInt(size, 10))
	c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderContentSecurityPolicy, "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	data, err := io.ReadAll(f)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).Send(data)
}
