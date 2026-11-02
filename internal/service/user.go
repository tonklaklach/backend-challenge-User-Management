package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chayupon/backend-challenge/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type TokenIssuer interface {
	Issue(string) (string, int64, error)
}

type User struct {
	repo      domain.UserRepository
	tokens    TokenIssuer
	dummyHash []byte
}

func NewUser(repo domain.UserRepository, tokens TokenIssuer) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte("missing-account-password"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("initialize password comparison: %w", err)
	}
	user := &User{
		repo:      repo,
		tokens:    tokens,
		dummyHash: hash,
	}

	return user, nil
}

func validName(name string) bool {
	return name != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= 100
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		return "", fmt.Errorf("%w: invalid email", domain.ErrInvalidInput)
	}
	return email, nil
}

func validID(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func (s *User) Create(ctx context.Context, name, email, password string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if !validName(name) || len(password) < 8 || len(password) > 72 {
		return domain.User{}, fmt.Errorf("%w: name must contain 1–100 characters; password must contain 8–72 bytes", domain.ErrInvalidInput)
	}
	email, err := normalizeEmail(email)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}
	user := domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}
	response, err := s.repo.Create(ctx, user)
	return response, err
}

func (s *User) Login(ctx context.Context, email, password string) (string, int64, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) == 0 || len(password) > 72 {
		return "", 0, domain.ErrInvalidCredentials
	}
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", 0, err
	}
	hash := []byte(user.PasswordHash)
	if errors.Is(err, domain.ErrNotFound) {
		hash = s.dummyHash
	}
	compareErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil || compareErr != nil {
		return "", 0, domain.ErrInvalidCredentials
	}
	token, expiresIn, err := s.tokens.Issue(user.ID)
	return token, expiresIn, err
}

func (s *User) Get(ctx context.Context, id string) (domain.User, error) {
	if !validID(id) {
		return domain.User{}, fmt.Errorf("%w: invalid user ID", domain.ErrInvalidInput)
	}
	response, err := s.repo.GetByID(ctx, id)
	return response, err
}

func (s *User) List(ctx context.Context) ([]domain.User, error) {
	response, err := s.repo.List(ctx)
	return response, err
}

func (s *User) Update(ctx context.Context, actorID, id string, update domain.UserUpdate) (domain.User, error) {
	id = strings.ToLower(id)
	if !validID(id) {
		return domain.User{}, fmt.Errorf("%w: invalid user ID", domain.ErrInvalidInput)
	}
	if actorID == "" || actorID != id {
		return domain.User{}, domain.ErrForbidden
	}
	if update.Name == nil && update.Email == nil {
		return domain.User{}, fmt.Errorf("%w: provide name or email", domain.ErrInvalidInput)
	}
	if update.Name != nil {
		name := strings.TrimSpace(*update.Name)
		if !validName(name) {
			return domain.User{}, fmt.Errorf("%w: invalid name", domain.ErrInvalidInput)
		}
		update.Name = &name
	}
	if update.Email != nil {
		email, err := normalizeEmail(*update.Email)
		if err != nil {
			return domain.User{}, err
		}
		update.Email = &email
	}
	response, err := s.repo.Update(ctx, id, update)
	return response, err
}

func (s *User) Delete(ctx context.Context, actorID, id string) error {
	id = strings.ToLower(id)
	if !validID(id) {
		return fmt.Errorf("%w: invalid user ID", domain.ErrInvalidInput)
	}
	if actorID == "" || actorID != id {
		return domain.ErrForbidden
	}
	err := s.repo.Delete(ctx, id)
	return err
}
