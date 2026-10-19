package repository

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestIsDuplicateKeyError(t *testing.T) {
	duplicate := mongo.CommandError{Code: 11000, Message: "duplicate key"}
	if !isDuplicateKeyError(duplicate) {
		t.Fatal("expected duplicate-key index build error to be recognized")
	}
	if !errors.Is(historicalDuplicateEmailWarning(duplicate), ErrHistoricalDuplicateEmails) {
		t.Fatal("expected duplicate-data warning to preserve its sentinel")
	}
	if isDuplicateKeyError(errors.New("network unavailable")) {
		t.Fatal("non-duplicate index errors must remain fatal")
	}
}

func TestIsIndexNotFoundError(t *testing.T) {
	if !isIndexNotFoundError(mongo.CommandError{Code: 27, Message: "index not found"}) {
		t.Fatal("expected index-not-found error to be recognized")
	}
	if isIndexNotFoundError(errors.New("network unavailable")) {
		t.Fatal("non-index errors must remain fatal")
	}
}

func TestEnsureIndexesCreatesDeterministicBlockSortIndex(t *testing.T) {
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("set MONGO_TEST_URI to run MongoDB integration tests")
	}
	ctx := context.Background()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })
	if _, err := EnsureIndexes(ctx, client); err != nil {
		t.Fatal(err)
	}

	cursor, err := client.Database("Auth").Collection("Block").Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var indexes []struct {
		Name string `bson:"name"`
		Key  bson.D `bson:"key"`
	}
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatal(err)
	}
	want := bson.D{{Key: "pageId", Value: int32(1)}, {Key: "order", Value: int32(1)}, {Key: "_id", Value: int32(1)}}
	for _, index := range indexes {
		if index.Name == "block_page_order_id" {
			if !reflect.DeepEqual(index.Key, want) {
				t.Fatalf("block sort index keys = %v, want %v", index.Key, want)
			}
			return
		}
	}
	t.Fatal("block_page_order_id index was not created")
}
