package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"svc-registry/internal/links"
	"svc-registry/internal/service"
	"svc-registry/internal/uploads"
)

func (a *API) UploadLinkIcon(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	header, err := c.FormFile("file")
	if err != nil {
		return Fail(c, NewAPIError(http.StatusBadRequest, "validation.invalid_body", "a multipart form with a file field is required"))
	}
	tooLarge := NewAPIError(http.StatusRequestEntityTooLarge, "validation.icon_too_large", "the icon is larger than 64 KiB")
	if header.Size > links.MaxIconBytes {
		return Fail(c, tooLarge)
	}
	f, err := header.Open()
	if err != nil {
		return Fail(c, NewAPIError(http.StatusBadRequest, "validation.invalid_body", "the file cannot be read"))
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, links.MaxIconBytes+1))
	if err != nil {
		return Fail(c, NewAPIError(http.StatusBadRequest, "validation.invalid_body", "the file cannot be read"))
	}
	if len(data) > links.MaxIconBytes {
		return Fail(c, tooLarge)
	}
	icon, err := service.UploadLinkIcon(c.Context(), a.State, p, id, data)
	if errors.Is(err, links.ErrIconTooLarge) {
		return Fail(c, tooLarge)
	}
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"id": icon, "url": "/api/v1/link-icons/" + icon})
}

func (a *API) LinkIcon(c fiber.Ctx) error {
	if _, _, err := a.currentUser(c); err != nil {
		return Fail(c, err)
	}
	id := c.Params("id")
	f, size, err := service.OpenLinkIcon(a.State, id)
	if err != nil {
		return Fail(c, err)
	}
	defer f.Close()
	c.Set(fiber.HeaderContentType, uploads.ContentType(id))
	c.Set(fiber.HeaderContentLength, strconv.FormatInt(size, 10))
	c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderContentSecurityPolicy, "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	data, err := io.ReadAll(f)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).Send(data)
}
