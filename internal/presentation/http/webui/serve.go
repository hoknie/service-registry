package webui

import (
	"net/http"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
)

const localeCookieMaxAge = 31_536_000

func Serve(dist Dist) fiber.Handler {
	return func(c fiber.Ctx) error {
		method := c.Method()
		if method != fiber.MethodGet && method != fiber.MethodHead {
			c.Set(fiber.HeaderAllow, "GET, HEAD")
			c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
			return c.Status(http.StatusMethodNotAllowed).SendString("method not allowed\n")
		}
		req := c.Request()
		raw := string(req.URI().PathOriginal())
		query := ""
		if q := req.URI().QueryString(); len(q) > 0 {
			query = "?" + string(q)
		}

		if len(raw) > 1 && strings.HasSuffix(raw, "/") {
			trimmed := raw[:len(raw)-1]
			if _, err := parsePath(trimmed); err != nil {
				return sendBadPath(c)
			}
			return redirect(c, http.StatusPermanentRedirect, trimmed+query)
		}
		segments, err := parsePath(raw)
		if err != nil {
			return sendBadPath(c)
		}

		remembered, _ := asLocale(c.Cookies(LocaleCookie))
		prefix, hasPrefix := "", false
		if len(segments) > 0 {
			prefix, hasPrefix = pathLocale(segments[0])
		}
		if !hasPrefix && isPagePath(segments) {
			rest := raw
			if raw == "/" {
				rest = ""
			}
			return redirect(c, http.StatusTemporaryRedirect, "/"+negotiate(c, remembered)+rest+query)
		}

		locale := prefix
		if !hasPrefix {
			locale = negotiate(c, remembered)
		}
		ae := c.Get(fiber.HeaderAcceptEncoding) != ""
		w := want{
			br:     ae && c.AcceptsEncodings("br") != "",
			gzip:   ae && c.AcceptsEncodings("gzip") != "",
			locale: locale,
		}
		head := method == fiber.MethodHead
		res := lookup(dist.Root(), segments, w)
		if res.noExport {
			return sendUnavailable(c, locale)
		}
		if hasPrefix && remembered != prefix {
			cookie := http.Cookie{Name: LocaleCookie, Value: prefix, Path: "/", MaxAge: localeCookieMaxAge, SameSite: http.SameSiteLaxMode}
			c.Append(fiber.HeaderSetCookie, cookie.String())
		}
		if res.file != nil {
			return sendFile(c, res.file, cacheControl(segments), head)
		}
		if res.page == nil {
			return sendBareNotFound(c, locale)
		}
		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
		c.Status(http.StatusNotFound)
		if head {
			c.Response().Header.SetContentLength(len(res.page))
			return nil
		}
		return c.Send(res.page)
	}
}

func isPagePath(segments []string) bool {
	if len(segments) == 0 {
		return true
	}
	if segments[0] == "_next" {
		return false
	}
	return !strings.Contains(segments[len(segments)-1], ".")
}

func negotiate(c fiber.Ctx, remembered string) string {
	if remembered != "" {
		return remembered
	}
	values := c.Request().Header.PeekAll(fiber.HeaderAcceptLanguage)
	if len(values) == 0 {
		return PickLocale("", false)
	}
	return PickLocale(string(values[0]), true)
}

func sendFile(c fiber.Ctx, f *found, cache string, head bool) error {
	status := http.StatusOK
	if f.notFoundPage {
		status, cache = http.StatusNotFound, "no-cache"
	}
	c.Set(fiber.HeaderCacheControl, cache)
	c.Set(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
	c.Set(fiber.HeaderETag, f.etag)
	if !f.notFoundPage && c.Fresh() {
		c.Status(http.StatusNotModified)
		return nil
	}
	c.Status(status)
	c.Set(fiber.HeaderContentType, f.contentType)
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	if f.encoding != "" {
		c.Set(fiber.HeaderContentEncoding, f.encoding)
	}
	if head {
		c.Response().Header.SetContentLength(int(f.size))
		return nil
	}
	body, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}
	return c.Send(body)
}

func redirect(c fiber.Ctx, status int, location string) error {
	for i := 0; i < len(location); i++ {
		if b := location[i]; b < 0x20 || b == 0x7f {
			return sendBadPath(c)
		}
	}
	c.Set(fiber.HeaderLocation, location)
	c.Status(status)
	return nil
}
