package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"
)

func Logging(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			r := c.Request()
			started := time.Now()
			defer func() {
				logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
			}()
			err := next(c)
			return err
		}
	}
}
