package webui

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

func isPublicDocsPath(segments []string) bool {
	return len(segments) > 1 && segments[0] == "public" && segments[1] == "docs"
}

func publicDocsWithoutLocale(raw, locale string) string {
	return strings.TrimPrefix(raw, "/"+locale)
}

func servePublicPage(c fiber.Ctx, dist Dist, w want, head bool) error {
	res := lookup(dist.Root(), []string{w.locale, "public", "docs"}, w)
	if res.noExport {
		return sendUnavailable(c, w.locale)
	}
	if res.file != nil {
		err := sendFile(c, res.file, "no-cache", head)
		c.Append(fiber.HeaderVary, "Cookie", fiber.HeaderAcceptLanguage)
		return err
	}
	c.Append(fiber.HeaderVary, "Cookie", fiber.HeaderAcceptLanguage)
	if res.page == nil {
		return sendBareNotFound(c, w.locale)
	}
	return sendNotFoundPage(c, res.page, head)
}

func ServePublicHost(dist Dist) fiber.Handler {
	return func(c fiber.Ctx) error {
		method := c.Method()
		if method != fiber.MethodGet && method != fiber.MethodHead {
			return c.Next()
		}
		segments, err := parsePath(string(c.Request().URI().PathOriginal()))
		if err != nil {
			return sendBadPath(c)
		}
		if !isPagePath(segments) {
			return c.Next()
		}
		remembered, _ := asLocale(c.Cookies(LocaleCookie))
		ae := c.Get(fiber.HeaderAcceptEncoding) != ""
		w := want{br: ae && c.AcceptsEncodings("br") != "", gzip: ae && c.AcceptsEncodings("gzip") != "", locale: negotiate(c, remembered)}
		return servePublicPage(c, dist, w, method == fiber.MethodHead)
	}
}
