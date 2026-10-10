package webui

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

func sendUnavailable(c fiber.Ctx, locale string) error {
	lang, title, text := "en", "Web interface is unavailable",
		"The service is running, but the files of its web interface are missing. Try again in a moment. Administrators: check WEB_DIST_DIR."
	switch locale {
	case "es":
		lang, title, text = "es", "La interfaz web no está disponible",
			"El servicio está en funcionamiento, pero no se encuentran los archivos de su interfaz web. Inténtalo de nuevo en unos momentos. Administradores: comprueben WEB_DIST_DIR."
	case "ru":
		lang, title, text = "ru", "Веб-интерфейс недоступен",
			"Сервис работает, но файлы его веб-интерфейса не найдены. Повторите попытку через минуту. Администратору: проверьте WEB_DIST_DIR."
	case "zh":
		lang, title, text = "zh", "网页界面不可用",
			"服务正在运行，但找不到其网页界面的文件。请稍后再试。管理员：请检查 WEB_DIST_DIR。"
	}
	return sendHTML(c, http.StatusServiceUnavailable, "no-store",
		`<!doctype html><html lang="`+lang+`"><head><meta charset="utf-8">`+
			`<meta name="viewport" content="width=device-width,initial-scale=1"><title>`+title+`</title>`+
			`<style>body{font-family:system-ui,sans-serif;max-width:36rem;margin:15vh auto;padding:0 1rem;color:#222}</style>`+
			`</head><body><h1>`+title+`</h1><p>`+text+`</p></body></html>`)
}

func sendBareNotFound(c fiber.Ctx, locale string) error {
	return sendHTML(c, http.StatusNotFound, "no-cache",
		`<!doctype html><html lang="`+locale+`"><head><meta charset="utf-8"><title>404</title></head><body><h1>404</h1></body></html>`)
}

func sendBadPath(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.Status(http.StatusBadRequest).SendString("bad request\n")
}

func sendHTML(c fiber.Ctx, status int, cache, body string) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, cache)
	return c.Status(status).SendString(body)
}
