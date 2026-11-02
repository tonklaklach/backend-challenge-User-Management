package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("user not found")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrForbidden          = errors.New("cannot modify another user")
)

type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type UserUpdate struct {
	Name  *string
	Email *string
}

type UserRepository interface {
	Create(context.Context, User) (User, error)
	GetByID(context.Context, string) (User, error)
	GetByEmail(context.Context, string) (User, error)
	List(context.Context) ([]User, error)
	Update(context.Context, string, UserUpdate) (User, error)
	Delete(context.Context, string) error
	Count(context.Context) (int64, error)
}
