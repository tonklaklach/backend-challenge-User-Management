package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chayupon/backend-challenge/internal/auth"
	"github.com/labstack/echo/v5"
)

func TestAuthenticateSetsSubject(t *testing.T) {
	tokens, err := auth.NewTokens("0123456789abcdef0123456789abcdef", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := tokens.Issue("user-id")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	next := func(c *echo.Context) error {
		r := c.Request()
		called = true
		if auth.Subject(r.Context()) != "user-id" {
			t.Fatal("missing authenticated subject")
		}
		return c.NoContent(http.StatusNoContent)
	}
	r := httptest.NewRequest("GET", "/users", nil)
	r.Header.Set("Authorization", "bearer "+token)
	w := httptest.NewRecorder()
	if err := Authenticate(tokens)(next)(echo.New().NewContext(r, w)); err != nil {
		t.Fatal(err)
	}
	if !called || w.Code != http.StatusNoContent {
		t.Fatalf("status=%d called=%v", w.Code, called)
	}
}

func TestTimeoutPropagatesDeadlineAndCancellation(t *testing.T) {
	r := httptest.NewRequest("GET", "/users", nil)
	var requestDone <-chan struct{}
	err := Timeout(time.Second)(func(c *echo.Context) error {
		r := c.Request()
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Fatal("deadline missing")
		}
		requestDone = r.Context().Done()
		return nil
	})(echo.New().NewContext(r, httptest.NewRecorder()))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestDone:
	default:
		t.Fatal("request context not canceled after completion")
	}
}

func TestLoggingDoesNotIncludeCredentials(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	r := httptest.NewRequest("POST", "/auth/login?password=query-secret", strings.NewReader(`{"password":"body-secret"}`))
	r.Header.Set("Authorization", "Bearer token-secret")
	if err := Logging(logger)(func(c *echo.Context) error { return c.NoContent(http.StatusOK) })(echo.New().NewContext(r, httptest.NewRecorder())); err != nil {
		t.Fatal(err)
	}
	logs := output.String()
	for _, expected := range []string{`"method":"POST"`, `"path":"/auth/login"`, `"duration":`} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("missing %s", expected)
		}
	}
	for _, secret := range []string{"query-secret", "body-secret", "token-secret"} {
		if strings.Contains(logs, secret) {
			t.Fatal("credential logged")
		}
	}
}
