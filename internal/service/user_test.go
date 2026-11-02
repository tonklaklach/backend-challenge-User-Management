package service

import (
	"context"
	"errors"
	"testing"

	"github.com/chayupon/backend-challenge/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const userID = "507f1f77bcf86cd799439011"
const otherID = "507f1f77bcf86cd799439012"

type fakeRepository struct {
	domain.UserRepository
	create     func(context.Context, domain.User) (domain.User, error)
	getByEmail func(context.Context, string) (domain.User, error)
	update     func(context.Context, string, domain.UserUpdate) (domain.User, error)
	delete     func(context.Context, string) error
}

func (r fakeRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	return r.create(ctx, user)
}
func (r fakeRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.getByEmail(ctx, email)
}
func (r fakeRepository) Update(ctx context.Context, id string, update domain.UserUpdate) (domain.User, error) {
	return r.update(ctx, id, update)
}
func (r fakeRepository) Delete(ctx context.Context, id string) error { return r.delete(ctx, id) }

type fakeTokens struct{}

func (fakeTokens) Issue(id string) (string, int64, error) { return "token:" + id, 3600, nil }

func newTestService(t *testing.T, repo domain.UserRepository) *User {
	t.Helper()
	s, err := NewUser(repo, fakeTokens{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateNormalizesAndHashesPassword(t *testing.T) {
	s := newTestService(t, fakeRepository{create: func(_ context.Context, user domain.User) (domain.User, error) {
		if user.Name != "Alice" || user.Email != "alice@example.com" {
			t.Fatalf("unexpected user: name=%q email=%q", user.Name, user.Email)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
			t.Fatal("password was not hashed correctly")
		}
		if user.CreatedAt.IsZero() {
			t.Fatal("missing creation time")
		}
		user.ID = userID
		return user, nil
	}})
	user, err := s.Create(context.Background(), " Alice ", " ALICE@EXAMPLE.COM ", "password123")
	if err != nil || user.ID != userID {
		t.Fatalf("id=%s err=%v", user.ID, err)
	}
}

func TestCreateValidation(t *testing.T) {
	s := newTestService(t, fakeRepository{})
	for _, tc := range []struct{ name, email, password string }{
		{" ", "alice@example.com", "password123"}, {"Alice", "bad", "password123"},
		{"Alice", "Alice <alice@example.com>", "password123"}, {"Alice", "alice@example.com", "short"},
		{"Alice", "alice@example.com", string(make([]byte, 73))},
	} {
		_, err := s.Create(context.Background(), tc.name, tc.email, tc.password)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	}
}

func TestCreatePropagatesConflict(t *testing.T) {
	s := newTestService(t, fakeRepository{create: func(context.Context, domain.User) (domain.User, error) { return domain.User{}, domain.ErrEmailExists }})
	_, err := s.Create(context.Background(), "Alice", "alice@example.com", "password123")
	if !errors.Is(err, domain.ErrEmailExists) {
		t.Fatalf("got %v", err)
	}
}

func TestLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	storageErr := errors.New("storage unavailable")
	for _, tc := range []struct {
		name, password     string
		lookupErr, wantErr error
	}{
		{"success", "password123", nil, nil}, {"wrong password", "wrong", nil, domain.ErrInvalidCredentials},
		{"missing account", "password123", domain.ErrNotFound, domain.ErrInvalidCredentials},
		{"database failure", "password123", storageErr, storageErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t, fakeRepository{getByEmail: func(_ context.Context, email string) (domain.User, error) {
				if email != "alice@example.com" {
					t.Fatalf("email=%q", email)
				}
				return domain.User{ID: userID, PasswordHash: string(hash)}, tc.lookupErr
			}})
			token, ttl, err := s.Login(context.Background(), " ALICE@example.com ", tc.password)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v want %v", err, tc.wantErr)
			}
			if err == nil && (token != "token:"+userID || ttl != 3600) {
				t.Fatal("unexpected token")
			}
		})
	}
}

func TestMutationRequiresOwner(t *testing.T) {
	s := newTestService(t, fakeRepository{})
	name := "Alice"
	for _, actor := range []string{"", otherID} {
		if _, err := s.Update(context.Background(), actor, userID, domain.UserUpdate{Name: &name}); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("update: %v", err)
		}
		if err := s.Delete(context.Background(), actor, userID); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("delete: %v", err)
		}
	}
}

func TestUpdateOnlySuppliedFields(t *testing.T) {
	s := newTestService(t, fakeRepository{update: func(_ context.Context, id string, update domain.UserUpdate) (domain.User, error) {
		if id != userID || update.Name != nil || update.Email == nil || *update.Email != "new@example.com" {
			t.Fatal("incorrect update")
		}
		return domain.User{ID: id, Email: *update.Email}, nil
	}})
	email := " NEW@example.com "
	if _, err := s.Update(context.Background(), userID, userID, domain.UserUpdate{Email: &email}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(context.Background(), userID, userID, domain.UserUpdate{}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty update: %v", err)
	}
	name := " "
	if _, err := s.Update(context.Background(), userID, userID, domain.UserUpdate{Name: &name}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("blank name: %v", err)
	}
}

func TestDeleteOwnerAndNotFound(t *testing.T) {
	s := newTestService(t, fakeRepository{delete: func(_ context.Context, id string) error {
		if id != userID {
			t.Fatal("wrong ID")
		}
		return domain.ErrNotFound
	}})
	if err := s.Delete(context.Background(), userID, userID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Delete(context.Background(), userID, "bad"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatal(err)
	}
}
