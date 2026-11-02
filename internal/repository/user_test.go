package repository

import (
	"errors"
	"testing"

	"github.com/chayupon/backend-challenge/internal/domain"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestPersistenceError(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name        string
		input, want error
	}{
		{"not found", mongo.ErrNoDocuments, domain.ErrNotFound},
		{"duplicate", mongo.WriteException{WriteErrors: mongo.WriteErrors{{Code: 11000, Message: "duplicate email"}}}, domain.ErrEmailExists},
		{"unexpected", databaseErr, databaseErr}, {"nil", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := persistenceError(tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
