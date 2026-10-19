package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrHistoricalDuplicateEmails = errors.New("unique email index was not created because historical exact duplicate emails exist")

func EnsureIndexes(ctx context.Context, client *mongo.Client) ([]error, error) {
	database := client.Database("Auth")
	var warnings []error

	if _, err := database.Collection("User").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetName("user_email_unique").SetUnique(true),
	}); err != nil {
		if isDuplicateKeyError(err) {
			warnings = append(warnings, historicalDuplicateEmailWarning(err))
		} else {
			return warnings, err
		}
	}

	if _, err := database.Collection("Page").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetName("page_slug_unique").SetUnique(true).SetPartialFilterExpression(
				bson.M{"slug": bson.M{"$type": "string"}},
			),
		},
		{
			Keys:    bson.D{{Key: "ownerId", Value: 1}, {Key: "deletedAt", Value: 1}, {Key: "order", Value: 1}},
			Options: options.Index().SetName("page_owner_order"),
		},
		{
			Keys:    bson.D{{Key: "parentPageId", Value: 1}},
			Options: options.Index().SetName("page_parent"),
		},
	}); err != nil {
		return warnings, err
	}

	_, err := database.Collection("Block").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "pageId", Value: 1}, {Key: "order", Value: 1}, {Key: "_id", Value: 1}},
		Options: options.Index().SetName("block_page_order_id"),
	})
	if err != nil {
		return warnings, err
	}
	err = database.Collection("Block").Indexes().DropOne(ctx, "block_page_order")
	if err != nil && !isIndexNotFoundError(err) {
		return warnings, err
	}
	return warnings, nil
}

func isIndexNotFoundError(err error) bool {
	var serverError mongo.ServerError
	return errors.As(err, &serverError) && serverError.HasErrorCode(27)
}

func historicalDuplicateEmailWarning(err error) error {
	return fmt.Errorf("%w: %v", ErrHistoricalDuplicateEmails, err)
}

func isDuplicateKeyError(err error) bool {
	if mongo.IsDuplicateKeyError(err) {
		return true
	}
	var serverError mongo.ServerError
	return errors.As(err, &serverError) && serverError.HasErrorCode(11000)
}
