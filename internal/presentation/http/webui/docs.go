package webui

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

const DocsSegment = "doc"

func isDocsPath(segments []string) bool {
	return len(segments) > 0 && segments[0] == DocsSegment
}

func docsWithoutLocale(raw, locale string) string {
	return strings.TrimPrefix(raw, "/"+locale)
}

func serveDocs(c fiber.Ctx, dist Dist, locale string, w want, head bool) error {
	res := lookup(dist.Root(), []string{locale, DocsSegment}, w)
	if res.noExport {
		return sendUnavailable(c, locale)
	}
	if res.file != nil {
		err := sendFile(c, res.file, "no-cache", head)
		c.Append(fiber.HeaderVary, "Cookie", fiber.HeaderAcceptLanguage)
		return err
	}
	c.Append(fiber.HeaderVary, "Cookie", fiber.HeaderAcceptLanguage)
	if res.page == nil {
		return sendBareNotFound(c, locale)
	}
	return sendNotFoundPage(c, res.page, head)
}
