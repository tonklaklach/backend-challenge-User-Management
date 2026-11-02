package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestTokenRoundTrip(t *testing.T) {
	tokens, err := NewTokens(testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	raw, ttl, err := tokens.Issue("user-id")
	if err != nil || ttl != 3600 {
		t.Fatalf("ttl=%d err=%v", ttl, err)
	}
	subject, err := tokens.Verify(raw)
	if err != nil || subject != "user-id" {
		t.Fatalf("subject=%q err=%v", subject, err)
	}
}

func TestRejectInvalidTokens(t *testing.T) {
	tokens, err := NewTokens(testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		secret string
		claims jwt.RegisteredClaims
	}{
		{"expired", jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "id", Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}},
		{"missing expiry", jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "id", Issuer: issuer}},
		{"wrong algorithm", jwt.SigningMethodHS384, testSecret, jwt.RegisteredClaims{Subject: "id", Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{"wrong key", jwt.SigningMethodHS256, "another-secret", jwt.RegisteredClaims{Subject: "id", Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{"missing subject", jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Issuer: issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
		{"wrong issuer", jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "id", Issuer: "other", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString([]byte(tc.secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tokens.Verify(raw); err == nil {
				t.Fatal("accepted invalid token")
			}
		})
	}
	if _, err := tokens.Verify("malformed"); err == nil {
		t.Fatal("accepted malformed token")
	}
}

func TestRejectInvalidConfiguration(t *testing.T) {
	if _, err := NewTokens("short", time.Hour); err == nil {
		t.Fatal("accepted short secret")
	}
	if _, err := NewTokens(testSecret, 0); err == nil {
		t.Fatal("accepted invalid ttl")
	}
}
