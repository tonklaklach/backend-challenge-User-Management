package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "backend-challenge"

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret string, ttl time.Duration) (*Tokens, error) {
	if len(secret) < 32 || ttl < time.Second {
		return nil, errors.New("JWT secret must be at least 32 bytes and TTL at least one second")
	}
	tokens := &Tokens{
		secret: []byte(secret),
		ttl:    ttl,
	}
	return tokens, nil
}

func (t *Tokens) Issue(subject string) (string, int64, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	expiresIn := int64(t.ttl.Seconds())
	return signed, expiresIn, err
}

func (t *Tokens) Verify(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(), jwt.WithIssuer(issuer), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", errors.New("invalid token")
	}
	return claims.Subject, nil
}

type subjectKey struct{}

func WithSubject(ctx context.Context, subject string) context.Context {
	authenticatedCtx := context.WithValue(ctx, subjectKey{}, subject)
	return authenticatedCtx
}

func Subject(ctx context.Context) string {
	subject, _ := ctx.Value(subjectKey{}).(string)
	return subject
}
