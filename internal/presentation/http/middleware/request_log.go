package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func RequestLog() fiber.Handler {
	return logger.New(logger.Config{
		Format: "${latency}",
		LoggerFunc: func(c fiber.Ctx, data *logger.Data, _ *logger.Config) error {
			path := string(c.Request().URI().PathOriginal())
			level, msg := slog.LevelDebug, "web"
			if path == "/api" || strings.HasPrefix(path, "/api/") {
				level, msg = slog.LevelInfo, "request"
			}
			slog.Log(c.Context(), level, msg, "method", c.Method(), "path", path,
				"status", c.Response().StatusCode(), "ms", data.Stop.Sub(data.Start).Milliseconds())
			return nil
		},
	})
}
