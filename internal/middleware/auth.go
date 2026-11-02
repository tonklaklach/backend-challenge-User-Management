package middleware

import (
	"net/http"
	"strings"

	"github.com/chayupon/backend-challenge/internal/auth"
	"github.com/chayupon/backend-challenge/internal/model"
	"github.com/labstack/echo/v5"
)

type TokenVerifier interface{ Verify(string) (string, error) }

func Authenticate(tokens TokenVerifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			r := c.Request()
			fields := strings.Fields(r.Header.Get("Authorization"))
			if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
				if subject, err := tokens.Verify(fields[1]); err == nil {
					c.SetRequest(r.WithContext(auth.WithSubject(r.Context(), subject)))
					err = next(c)
					return err
				}
			}
			c.Response().Header().Set("WWW-Authenticate", "Bearer")
			response := model.ErrorResponse{
				Error: "valid bearer token required",
			}
			err := c.JSON(http.StatusUnauthorized, response)
			return err
		}
	}
}
