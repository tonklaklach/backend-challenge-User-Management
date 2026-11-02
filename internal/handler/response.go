package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/chayupon/backend-challenge/internal/domain"
	"github.com/chayupon/backend-challenge/internal/model"
	"github.com/labstack/echo/v5"
)

func decode(c *echo.Context, target any) error {
	// Decode JSON directly to preserve body-only input and reject unknown fields.
	r := c.Request()
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(c.Response(), r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(target)
	if err == nil {
		var extra any
		trailingErr := decoder.Decode(&extra)
		switch {
		case trailingErr == nil:
			err = errors.New("unexpected JSON content")
		case trailingErr != io.EOF:
			err = trailingErr
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		status := http.StatusBadRequest
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		return echo.NewHTTPError(status, "invalid JSON request")
	}
	return nil
}

func HTTPErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
			return
		}
		status, message := errorResponse(err)
		if status >= http.StatusInternalServerError {
			logger.Error("request failed", "error", err)
		}
		var writeErr error
		if c.Request().Method == http.MethodHead {
			writeErr = c.NoContent(status)
		} else {
			response := model.ErrorResponse{
				Error: message,
			}
			writeErr = c.JSON(status, response)
		}
		if writeErr != nil {
			logger.Error("write error response failed", "error", writeErr)
		}
	}
}

func errorResponse(err error) (int, string) {
	status, message := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrInvalidCredentials):
		status, message = http.StatusUnauthorized, domain.ErrInvalidCredentials.Error()
	case errors.Is(err, domain.ErrForbidden):
		status, message = http.StatusForbidden, domain.ErrForbidden.Error()
	case errors.Is(err, domain.ErrNotFound):
		status, message = http.StatusNotFound, domain.ErrNotFound.Error()
	case errors.Is(err, domain.ErrEmailExists):
		status, message = http.StatusConflict, domain.ErrEmailExists.Error()
	default:
		if code := echo.StatusCode(err); code >= 400 && code < 500 {
			status, message = code, http.StatusText(code)
			var httpErr *echo.HTTPError
			if errors.As(err, &httpErr) && httpErr.Message != "" {
				message = httpErr.Message
			}
		}
	}
	return status, message
}

func userResponse(user domain.User) model.UserResponse {
	response := model.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
	return response
}
