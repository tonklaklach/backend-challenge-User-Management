package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chayupon/backend-challenge/internal/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type userDocument struct {
	ID           bson.ObjectID `bson:"_id"`
	Name         string        `bson:"name"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"password_hash"`
	CreatedAt    time.Time     `bson:"created_at"`
}

func (d userDocument) user() domain.User {
	user := domain.User{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		CreatedAt:    d.CreatedAt,
	}
	return user
}

type User struct{ collection *mongo.Collection }

var _ domain.UserRepository = (*User)(nil)

func NewUser(database *mongo.Database) *User {
	user := &User{
		collection: database.Collection("users"),
	}
	return user
}

func (r *User) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create unique email index: %w", err)
	}
	return nil
}

func persistenceError(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrEmailExists
	}
	if err != nil {
		return fmt.Errorf("user repository: %w", err)
	}
	return nil
}

func (r *User) Create(ctx context.Context, user domain.User) (domain.User, error) {
	doc := userDocument{
		ID:           bson.NewObjectID(),
		Name:         user.Name,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		repositoryErr := persistenceError(err)
		return domain.User{}, repositoryErr
	}
	response := doc.user()
	return response, nil
}

func (r *User) GetByID(ctx context.Context, id string) (domain.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.User{}, domain.ErrInvalidInput
	}
	response, err := r.findOne(ctx, bson.M{"_id": objectID})
	return response, err
}

func (r *User) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	response, err := r.findOne(ctx, bson.M{"email": email})
	return response, err
}

func (r *User) findOne(ctx context.Context, filter bson.M) (domain.User, error) {
	var doc userDocument
	if err := r.collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		repositoryErr := persistenceError(err)
		return domain.User{}, repositoryErr
	}
	response := doc.user()
	return response, nil
}

func (r *User) List(ctx context.Context) ([]domain.User, error) {
	cursor, err := r.collection.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetProjection(bson.M{"password_hash": 0}))
	if err != nil {
		repositoryErr := persistenceError(err)
		return nil, repositoryErr
	}
	defer cursor.Close(ctx)
	users := make([]domain.User, 0)
	for cursor.Next(ctx) {
		var doc userDocument
		if err := cursor.Decode(&doc); err != nil {
			repositoryErr := persistenceError(err)
			return nil, repositoryErr
		}
		users = append(users, doc.user())
	}
	if err := cursor.Err(); err != nil {
		repositoryErr := persistenceError(err)
		return nil, repositoryErr
	}
	return users, nil
}

func (r *User) Update(ctx context.Context, id string, update domain.UserUpdate) (domain.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.User{}, domain.ErrInvalidInput
	}
	fields := bson.M{}
	if update.Name != nil {
		fields["name"] = *update.Name
	}
	if update.Email != nil {
		fields["email"] = *update.Email
	}
	var doc userDocument
	err = r.collection.FindOneAndUpdate(ctx, bson.M{"_id": objectID}, bson.M{"$set": fields}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err != nil {
		repositoryErr := persistenceError(err)
		return domain.User{}, repositoryErr
	}
	response := doc.user()
	return response, nil
}

func (r *User) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrInvalidInput
	}
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		repositoryErr := persistenceError(err)
		return repositoryErr
	}
	if result.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *User) Count(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.D{})
	repositoryErr := persistenceError(err)
	return count, repositoryErr
}
