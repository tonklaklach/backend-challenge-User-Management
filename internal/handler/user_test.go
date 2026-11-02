package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chayupon/backend-challenge/internal/domain"
	"github.com/labstack/echo/v5"
)

type stubService struct {
	UserService
	create func(context.Context, string, string, string) (domain.User, error)
	login  func(context.Context, string, string) (string, int64, error)
	list   func(context.Context) ([]domain.User, error)
}

func (s stubService) Create(ctx context.Context, name, email, password string) (domain.User, error) {
	return s.create(ctx, name, email, password)
}
func (s stubService) Login(ctx context.Context, email, password string) (string, int64, error) {
	return s.login(ctx, email, password)
}
func (s stubService) List(ctx context.Context) ([]domain.User, error) { return s.list(ctx) }

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestCreateResponse(t *testing.T) {
	h := NewUser(stubService{create: func(_ context.Context, name, email, password string) (domain.User, error) {
		if name != "Alice" || email != "alice@example.com" || password != "password123" {
			t.Fatal("incorrect request mapping")
		}
		return domain.User{ID: "id", Name: name, Email: email, PasswordHash: "secret-hash"}, nil
	}})
	r := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"name":"Alice","email":"alice@example.com","password":"password123"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveHandler(t, h.Create, w, r)
	if w.Code != http.StatusCreated || strings.Contains(w.Body.String(), "secret-hash") || strings.Contains(w.Body.String(), "password") {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
}

func TestLoginResponse(t *testing.T) {
	h := NewUser(stubService{login: func(_ context.Context, email, password string) (string, int64, error) {
		if email != "alice@example.com" || password != "password123" {
			t.Fatal("incorrect credentials mapping")
		}
		return "signed-token", 3600, nil
	}})
	r := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":"alice@example.com","password":"password123"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	serveHandler(t, h.Login, w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"access_token":"signed-token"`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
}

func TestListEmptyIsArray(t *testing.T) {
	h := NewUser(stubService{list: func(context.Context) ([]domain.User, error) { return nil, nil }})
	w := httptest.NewRecorder()
	serveHandler(t, h.List, w, httptest.NewRequest("GET", "/users", nil))
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
}

func TestErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{domain.ErrInvalidInput, 400}, {domain.ErrInvalidCredentials, 401}, {domain.ErrForbidden, 403},
		{domain.ErrNotFound, 404}, {domain.ErrEmailExists, 409}, {errors.New("private storage detail"), 500},
	} {
		w := httptest.NewRecorder()
		e := echo.New()
		HTTPErrorHandler(testLogger())(e.NewContext(httptest.NewRequest("GET", "/", nil), w), tc.err)
		if w.Code != tc.status {
			t.Fatalf("%v: got %d", tc.err, w.Code)
		}
		if strings.Contains(w.Body.String(), "private storage detail") {
			t.Fatal("internal error leaked")
		}
	}
}

func serveHandler(t *testing.T, handler echo.HandlerFunc, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	e := echo.New()
	c := e.NewContext(r, w)
	if err := handler(c); err != nil {
		HTTPErrorHandler(testLogger())(c, err)
	}
}

func TestHTTPErrorHandlerPreservesCommittedResponse(t *testing.T) {
	e := echo.New()
	w := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest("GET", "/", nil), w)
	if err := c.JSON(http.StatusAccepted, map[string]string{"result": "accepted"}); err != nil {
		t.Fatal(err)
	}
	before := w.Body.String()
	HTTPErrorHandler(testLogger())(c, errors.New("late error"))
	if w.Code != http.StatusAccepted || w.Body.String() != before {
		t.Fatal("committed response was overwritten")
	}
}

func TestDecodeBodyLimitBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bodySize int
		status   int
	}{
		{"exactly 1 MiB", 1 << 20, 0},
		{"one byte over 1 MiB", (1 << 20) + 1, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{}` + strings.Repeat(" ", tc.bodySize-2)
			r := httptest.NewRequest("POST", "/users/get", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			c := echo.New().NewContext(r, httptest.NewRecorder())
			var request struct {
				ID string `json:"id"`
			}
			err := decode(c, &request)
			if got := echo.StatusCode(err); got != tc.status || tc.status == 0 && err != nil {
				t.Fatalf("got status=%d error=%v, want status=%d", got, err, tc.status)
			}
		})
	}
}
