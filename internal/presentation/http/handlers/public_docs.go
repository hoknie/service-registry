package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	publishingservices "svc-registry/internal/feature/publishing/services"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/responses"
)

const PublicationHostKey = "publication_host"

func publicEntry(c fiber.Ctx) publishingservices.EntryRef {
	if id, ok := c.Locals(PublicationHostKey).(uuid.UUID); ok {
		return publishingservices.EntryRef{Host: &id}
	}
	return publishingservices.EntryRef{Root: c.Query("root")}
}

func publicSegments(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func publicVersion(c fiber.Ctx) publishingservices.VersionQuery {
	return publishingservices.VersionQuery{Branch: optionalQuery(c, "branch"), Tag: optionalQuery(c, "tag"), Release: optionalQuery(c, "release")}
}

func (a *Handlers) publicCall(c fiber.Ctx, fn func() (any, error)) error {
	if err := a.Publishing.Public.Allow(c.IP()); err != nil {
		return responses.Fail(c, err)
	}
	body, err := fn()
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "public, max-age=60")
	return c.Status(http.StatusOK).JSON(body)
}

func (a *Handlers) PublicSpace(c fiber.Ctx) error {
	return a.publicCall(c, func() (any, error) {
		entry := publicEntry(c)
		s, err := a.Publishing.Public.Space(c.Context(), entry, publicSegments(c.Query("path")))
		segment := entry.Root
		if entry.Host != nil && s.EntryHost != nil {
			segment = *s.EntryHost
		}
		return responses.PublicSpaceOf(s, segment), err
	})
}

func (a *Handlers) PublicTree(c fiber.Ctx) error {
	return a.publicCall(c, func() (any, error) {
		snap, level, err := a.Publishing.Public.Tree(c.Context(), publicEntry(c), publicSegments(c.Query("path")), publicVersion(c), c.Query("dir"))
		return responses.PublicTreeOf(snap, level), err
	})
}

func (a *Handlers) PublicFiles(c fiber.Ctx) error {
	return a.publicCall(c, func() (any, error) {
		snap, files, err := a.Publishing.Public.Files(c.Context(), publicEntry(c), publicSegments(c.Query("path")), publicVersion(c))
		return responses.PublicFilesOf(snap, files), err
	})
}

func (a *Handlers) PublicFile(c fiber.Ctx) error {
	return a.publicCall(c, func() (any, error) {
		snap, f, err := a.Publishing.Public.File(c.Context(), publicEntry(c), publicSegments(c.Query("path")), publicVersion(c), c.Query("file"))
		return responses.PublicFileOf(snap, f), err
	})
}

func (a *Handlers) PublicSearch(c fiber.Ctx) error {
	return a.publicCall(c, func() (any, error) {
		in := knowledgetypes.SearchInput{Q: c.Query("q"), Cursor: c.Query("cursor")}
		if v := optionalQuery(c, "limit"); v != nil {
			n, err := strconv.Atoi(*v)
			if err != nil {
				return nil, apperr.Wrap(knowledgetypes.InvalidSearch)
			}
			in.Limit = &n
		}
		hits, next, err := a.Publishing.Public.Search(c.Context(), publicEntry(c), publicSegments(c.Query("path")), publicVersion(c), in)
		return responses.PublicSearchOf(hits, next), err
	})
}
