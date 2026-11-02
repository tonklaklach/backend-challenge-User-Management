package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chayupon/backend-challenge/internal/auth"
	"github.com/chayupon/backend-challenge/internal/domain"
	"github.com/chayupon/backend-challenge/internal/handler"
	"github.com/chayupon/backend-challenge/internal/model"
	"github.com/labstack/echo/v5"
)

const routeUserID = "507f1f77bcf86cd799439011"

type stubUserService struct {
	handler.UserService
	get    func(context.Context, string) (domain.User, error)
	update func(context.Context, string, string, domain.UserUpdate) (domain.User, error)
	delete func(context.Context, string, string) error
}

func (s stubUserService) Get(ctx context.Context, id string) (domain.User, error) {
	return s.get(ctx, id)
}
func (s stubUserService) Update(ctx context.Context, actor, id string, update domain.UserUpdate) (domain.User, error) {
	return s.update(ctx, actor, id, update)
}
func (s stubUserService) Delete(ctx context.Context, actor, id string) error {
	return s.delete(ctx, actor, id)
}

func testRouter(t *testing.T, service handler.UserService) (*echo.Echo, string) {
	t.Helper()
	tokens, err := auth.NewTokens("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := tokens.Issue(routeUserID)
	if err != nil {
		t.Fatal(err)
	}
	mux := echo.New()
	mux.HTTPErrorHandler = handler.HTTPErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerRoutes(mux, handler.NewUser(service), tokens)
	return mux, token
}

func perform(mux http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestRoutesRequireAuthentication(t *testing.T) {
	mux, _ := testRouter(t, stubUserService{})
	for _, route := range []struct{ method, path string }{
		{"POST", "/users"}, {"POST", "/users/get"}, {"GET", "/users"}, {"PATCH", "/users"}, {"DELETE", "/users"},
	} {
		for _, token := range []string{"", "invalid"} {
			w := perform(mux, route.method, route.path, `{}`, token)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s: %d", route.method, route.path, w.Code)
			}
		}
	}
}

func TestGetUsesBodyAndHidesHash(t *testing.T) {
	mux, token := testRouter(t, stubUserService{get: func(_ context.Context, id string) (domain.User, error) {
		if id != routeUserID {
			t.Fatalf("wrong body ID %q", id)
		}
		return domain.User{ID: id, Name: "Alice", Email: "alice@example.com", PasswordHash: "sensitive-hash"}, nil
	}})
	w := perform(mux, "POST", "/users/get", `{"id":"`+routeUserID+`"}`, token)
	if w.Code != http.StatusOK {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "password") || strings.Contains(w.Body.String(), "sensitive-hash") {
		t.Fatal("password leaked")
	}
	var response model.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.ID != routeUserID {
		t.Fatalf("invalid response: %v", err)
	}
}

func TestRejectInvalidJSON(t *testing.T) {
	mux, token := testRouter(t, stubUserService{})
	for _, tc := range []struct {
		name   string
		body   string
		status int
	}{
		{"malformed JSON", `{`, http.StatusBadRequest},
		{"unknown field", `{"id":"x","password":"unexpected"}`, http.StatusBadRequest},
		{"multiple values", `{} {}`, http.StatusBadRequest},
		{"wrong field type", `{"id":10}`, http.StatusBadRequest},
		{"invalid syntax before limit", strings.Repeat("x", (1<<20)+1), http.StatusBadRequest},
		{"oversized JSON value", `{"id":"` + strings.Repeat("a", 1<<20) + `"}`, http.StatusRequestEntityTooLarge},
		{"oversized trailing whitespace", `{}` + strings.Repeat(" ", 1<<20), http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := perform(mux, "POST", "/users/get", tc.body, token)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
	r := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("got %d", w.Code)
	}
}

func TestUpdatePassesAuthenticatedActorAndBodyID(t *testing.T) {
	mux, token := testRouter(t, stubUserService{update: func(_ context.Context, actor, id string, update domain.UserUpdate) (domain.User, error) {
		if actor != routeUserID || id != routeUserID || update.Name == nil || *update.Name != "New name" || update.Email != nil {
			t.Fatal("incorrect update arguments")
		}
		return domain.User{}, domain.ErrForbidden
	}})
	w := perform(mux, "PATCH", "/users", `{"id":"`+routeUserID+`","name":"New name"}`, token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d", w.Code)
	}
}

func TestDeleteReturnsNoContent(t *testing.T) {
	mux, token := testRouter(t, stubUserService{delete: func(_ context.Context, actor, id string) error {
		if actor != routeUserID || id != routeUserID {
			t.Fatal("incorrect delete arguments")
		}
		return nil
	}})
	w := perform(mux, "DELETE", "/users", `{"id":"`+routeUserID+`"}`, token)
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
}

func TestRoutingErrorsUseJSON(t *testing.T) {
	router, _ := testRouter(t, stubUserService{})
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/unknown", http.StatusNotFound},
		{"GET", "/auth/login", http.StatusMethodNotAllowed},
	} {
		w := perform(router, tc.method, tc.path, "", "")
		if w.Code != tc.status {
			t.Fatalf("%s %s: got %d", tc.method, tc.path, w.Code)
		}
		var response model.ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Error == "" {
			t.Fatalf("invalid error response: %s", w.Body.String())
		}
	}
}
