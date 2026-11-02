package middleware

import (
	"context"
	"time"

	"github.com/labstack/echo/v5"
)

func Timeout(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			r := c.Request()
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			c.SetRequest(r.WithContext(ctx))
			err := next(c)
			return err
		}
	}
}
