//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/chayupon/backend-challenge/internal/db"
	"github.com/chayupon/backend-challenge/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func integrationRepository(t *testing.T) (*User, context.Context) {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Fatal("set MONGO_TEST_URI to a local/test MongoDB instance when running integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	client, err := db.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	// Each test owns a new database; cleanup never drops an existing application database.
	database := client.Database("backend_challenge_test_" + bson.NewObjectID().Hex())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := database.Drop(cleanupCtx); err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect test client: %v", err)
		}
	})
	repo := NewUser(database)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return repo, ctx
}

func integrationUser(name, email string) domain.User {
	user := domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: "stored-test-hash",
		CreatedAt:    time.Now().UTC().Truncate(time.Millisecond),
	}
	return user
}

func TestMongoUserLifecycle(t *testing.T) {
	repo, ctx := integrationRepository(t)
	users, err := repo.List(ctx)
	if err != nil || users == nil || len(users) != 0 {
		t.Fatalf("empty list: users=%v err=%v", users, err)
	}
	aliceInput := integrationUser("Alice", "alice@example.com")
	alice, err := repo.Create(ctx, aliceInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bson.ObjectIDFromHex(alice.ID); err != nil {
		t.Fatalf("invalid generated ID: %v", err)
	}
	stored, err := repo.GetByID(ctx, alice.ID)
	if err != nil || stored.Name != aliceInput.Name || stored.Email != aliceInput.Email || stored.PasswordHash != aliceInput.PasswordHash || !stored.CreatedAt.Equal(aliceInput.CreatedAt) {
		t.Fatalf("stored user did not round trip: id=%s err=%v", stored.ID, err)
	}
	byEmail, err := repo.GetByEmail(ctx, alice.Email)
	if err != nil || byEmail.ID != alice.ID {
		t.Fatalf("get by email: id=%s err=%v", byEmail.ID, err)
	}
	bob, err := repo.Create(ctx, integrationUser("Bob", "bob@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	users, err = repo.List(ctx)
	if err != nil || len(users) != 2 {
		t.Fatalf("list: count=%d err=%v", len(users), err)
	}
	if users[0].ID != alice.ID || users[1].ID != bob.ID {
		t.Fatal("list is not ordered by ID")
	}
	for _, user := range users {
		if user.PasswordHash != "" {
			t.Fatal("list query returned a password hash")
		}
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if _, err := repo.Create(ctx, integrationUser("Duplicate", alice.Email)); !errors.Is(err, domain.ErrEmailExists) {
		t.Fatalf("duplicate creation: %v", err)
	}
	name := "Alice Updated"
	updated, err := repo.Update(ctx, alice.ID, domain.UserUpdate{Name: &name})
	if err != nil || updated.Name != name || updated.Email != alice.Email || updated.PasswordHash != alice.PasswordHash || !updated.CreatedAt.Equal(alice.CreatedAt) {
		t.Fatalf("name update changed omitted fields or returned stale data: %v", err)
	}
	email := "alice.updated@example.com"
	updated, err = repo.Update(ctx, alice.ID, domain.UserUpdate{Email: &email})
	if err != nil || updated.Email != email || updated.Name != name {
		t.Fatalf("email update: %v", err)
	}
	if _, err := repo.GetByEmail(ctx, alice.Email); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old email still resolves: %v", err)
	}
	byEmail, err = repo.GetByEmail(ctx, email)
	if err != nil || byEmail.ID != alice.ID {
		t.Fatalf("updated email lookup: %v", err)
	}
	rejectedName := "Must not persist"
	if _, err := repo.Update(ctx, alice.ID, domain.UserUpdate{Name: &rejectedName, Email: &bob.Email}); !errors.Is(err, domain.ErrEmailExists) {
		t.Fatalf("duplicate email update: %v", err)
	}
	stored, err = repo.GetByID(ctx, alice.ID)
	if err != nil || stored.Name != name || stored.Email != email {
		t.Fatalf("failed update partially changed the user: %v", err)
	}
	missingID := bson.NewObjectID().Hex()
	if _, err := repo.GetByID(ctx, missingID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing user lookup: %v", err)
	}
	if _, err := repo.Update(ctx, missingID, domain.UserUpdate{Name: &name}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing user update: %v", err)
	}
	if err := repo.Delete(ctx, missingID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing user deletion: %v", err)
	}
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.GetByID(canceledCtx, alice.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request: %v", err)
	}
	if err := repo.Delete(ctx, alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, alice.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted user still exists: %v", err)
	}
	if err := repo.Delete(ctx, alice.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("repeated deletion: %v", err)
	}
	count, err = repo.Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("count after deletion=%d err=%v", count, err)
	}
}

func TestMongoConcurrentCreateEnforcesUniqueEmail(t *testing.T) {
	repo, ctx := integrationRepository(t)
	const requests = 12
	start := make(chan struct{})
	results := make(chan error, requests)
	for range requests {
		go func() {
			<-start
			_, err := repo.Create(ctx, integrationUser("Concurrent", "same@example.com"))
			results <- err
		}()
	}
	close(start)
	succeeded, conflicted := 0, 0
	for range requests {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrEmailExists):
			conflicted++
		default:
			t.Errorf("unexpected create error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != requests-1 {
		t.Fatalf("successes=%d conflicts=%d", succeeded, conflicted)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("stored users=%d err=%v", count, err)
	}
}

func TestMongoConcurrentUpdateEnforcesUniqueEmail(t *testing.T) {
	repo, ctx := integrationRepository(t)
	alice, err := repo.Create(ctx, integrationUser("Alice", "alice@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	bob, err := repo.Create(ctx, integrationUser("Bob", "bob@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	email := "claimed@example.com"
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{alice.ID, bob.ID} {
		go func() {
			<-start
			_, err := repo.Update(ctx, id, domain.UserUpdate{Email: &email})
			results <- err
		}()
	}
	close(start)
	succeeded, conflicted := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, domain.ErrEmailExists):
			conflicted++
		default:
			t.Errorf("unexpected update error: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("successes=%d conflicts=%d", succeeded, conflicted)
	}
	claimed, err := repo.GetByEmail(ctx, email)
	if err != nil || claimed.ID != alice.ID && claimed.ID != bob.ID {
		t.Fatalf("target email has no winning user: %v", err)
	}
	count, err := repo.Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("stored users=%d err=%v", count, err)
	}
}
