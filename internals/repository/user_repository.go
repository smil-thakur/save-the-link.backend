package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/smil-thakur/save-the-link/internals/crypto"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRepository struct {
	client          *mongo.Client
	passwordManager *crypto.PasswordManager
	ctx             context.Context
}

func NewUserRepository(client *mongo.Client, ctx context.Context, passwordManager *crypto.PasswordManager) *UserRepository {
	return &UserRepository{
		client:          client,
		ctx:             ctx,
		passwordManager: passwordManager,
	}
}

func (u *UserRepository) CheckUserExists(email string) (bool, error) {
	collection := u.client.Database("Auth").Collection("User")
	var user mongomodels.UserMongo
	err := collection.FindOne(u.ctx, bson.M{"email": email}).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (u *UserRepository) FindUserById(id string) (*mongomodels.UserMongo, error) {
	collection := u.client.Database("Auth").Collection("User")

	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return nil, customerrors.ErrorUserNotFound
	}

	var user mongomodels.UserMongo
	err = collection.FindOne(u.ctx, bson.M{"_id": objectId}).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (u *UserRepository) FindUserByEmail(email string) (*mongomodels.UserMongo, error) {
	collection := u.client.Database("Auth").Collection("User")

	var user mongomodels.UserMongo
	err := collection.FindOne(u.ctx, bson.M{"email": email}).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// SearchUsersByEmail is used for the collaborator autocomplete in the share
// dialog — a prefix match on email, excluding the requester themselves.
func (u *UserRepository) SearchUsersByEmail(query string, excludeUserId string, limit int64) ([]mongomodels.UserMongo, error) {
	collection := u.client.Database("Auth").Collection("User")

	filter := bson.M{
		"email": bson.M{"$regex": "^" + regexp.QuoteMeta(query), "$options": "i"},
	}

	if excludeObjectId, err := bson.ObjectIDFromHex(excludeUserId); err == nil {
		filter["_id"] = bson.M{"$ne": excludeObjectId}
	}

	cursor, err := collection.Find(u.ctx, filter, options.Find().SetLimit(limit))

	if err != nil {
		return nil, err
	}

	var users []mongomodels.UserMongo

	if err := cursor.All(u.ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func (u *UserRepository) DeleteUser(id string) error {
	collection := u.client.Database("Auth").Collection("User")

	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return customerrors.ErrorUserNotFound
	}

	_, err = collection.DeleteOne(u.ctx, bson.M{"_id": objectId})

	return err
}

func (u *UserRepository) LoginUser(email string, password string) (*mongomodels.UserMongo, error) {
	collection := u.client.Database("Auth").Collection("User")

	var user mongomodels.UserMongo
	err := collection.FindOne(u.ctx, bson.M{"email": email}).Decode(&user)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorUserNotFound
		}
		return nil, err
	}

	passwordMatch := u.passwordManager.ComparePassword(user.Password, password)

	if passwordMatch == true {
		return &user, nil
	}

	return nil, customerrors.InvalidCredentials
}

func (u *UserRepository) RegisterUserToDatabase(username string, email string, password string) (string, error) {

	exists, err := u.CheckUserExists(email)

	if err != nil {
		return "", err
	}

	if exists == true {
		return "", customerrors.UserAlreadyExists
	}

	collection := u.client.Database("Auth").Collection("User")

	hashedPassword, err := u.passwordManager.HashPassword(password)

	if err != nil {
		return "", err
	}

	result, err := collection.InsertOne(u.ctx, &mongomodels.UserMongo{
		Username: username,
		Email:    email,
		Password: hashedPassword,
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprint(result.InsertedID.(bson.ObjectID).Hex()), nil

}
