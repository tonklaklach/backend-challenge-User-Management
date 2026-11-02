package handler

import (
	"context"
	"net/http"

	"github.com/chayupon/backend-challenge/internal/auth"
	"github.com/chayupon/backend-challenge/internal/domain"
	"github.com/chayupon/backend-challenge/internal/model"
	"github.com/labstack/echo/v5"
)

type UserService interface {
	Create(context.Context, string, string, string) (domain.User, error)
	Login(context.Context, string, string) (string, int64, error)
	Get(context.Context, string) (domain.User, error)
	List(context.Context) ([]domain.User, error)
	Update(context.Context, string, string, domain.UserUpdate) (domain.User, error)
	Delete(context.Context, string, string) error
}

type User struct {
	service UserService
}

func NewUser(service UserService) *User {
	user := &User{
		service: service,
	}
	return user
}

func (h *User) Create(c *echo.Context) error {
	var request model.CreateUserRequest
	if err := decode(c, &request); err != nil {
		return err
	}
	user, err := h.service.Create(c.Request().Context(), request.Name, request.Email, request.Password)
	if err != nil {
		return err
	}
	response := userResponse(user)
	err = c.JSON(http.StatusCreated, response)
	return err
}

func (h *User) Login(c *echo.Context) error {
	var request model.LoginRequest
	if err := decode(c, &request); err != nil {
		return err
	}
	token, expiresIn, err := h.service.Login(c.Request().Context(), request.Email, request.Password)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	response := model.TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}
	err = c.JSON(http.StatusOK, response)
	return err
}

func (h *User) Get(c *echo.Context) error {
	var request model.UserIDRequest
	if err := decode(c, &request); err != nil {
		return err
	}
	user, err := h.service.Get(c.Request().Context(), request.ID)
	if err != nil {
		return err
	}
	response := userResponse(user)
	err = c.JSON(http.StatusOK, response)
	return err
}

func (h *User) List(c *echo.Context) error {
	users, err := h.service.List(c.Request().Context())
	if err != nil {
		return err
	}
	response := make([]model.UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, userResponse(user))
	}
	err = c.JSON(http.StatusOK, response)
	return err
}

func (h *User) Update(c *echo.Context) error {
	var request model.UpdateUserRequest
	if err := decode(c, &request); err != nil {
		return err
	}
	update := domain.UserUpdate{
		Name:  request.Name,
		Email: request.Email,
	}
	user, err := h.service.Update(c.Request().Context(), auth.Subject(c.Request().Context()), request.ID, update)
	if err != nil {
		return err
	}
	response := userResponse(user)
	err = c.JSON(http.StatusOK, response)
	return err
}

func (h *User) Delete(c *echo.Context) error {
	var request model.UserIDRequest
	if err := decode(c, &request); err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), auth.Subject(c.Request().Context()), request.ID); err != nil {
		return err
	}
	err := c.NoContent(http.StatusNoContent)
	return err
}
