package repository_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func blockRepositoryFixture(t *testing.T, title string) (*repository.BlockRepository, *mongo.Client, context.Context, bson.ObjectID, bson.ObjectID) {
	t.Helper()
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

	ownerID := bson.NewObjectID()
	pageRepository := repository.NewPageRepository(client, ctx)
	page, err := pageRepository.CreatePage(ownerID.Hex(), title, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": page.Id})
		_, _ = client.Database("Auth").Collection("Page").DeleteOne(ctx, bson.M{"_id": page.Id})
	})
	return repository.NewBlockRepository(client, ctx, pageRepository), client, ctx, ownerID, page.Id
}

func TestCreateBlockAssignsAtomicNewestFirstOrders(t *testing.T) {
	blockRepository, _, _, ownerID, pageID := blockRepositoryFixture(t, "newest first")

	first, err := blockRepository.CreateBlockForAuthorizedPage(pageID, &mongomodels.BlockMongo{URL: "https://first.example"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Order != -1 {
		t.Fatalf("first block order = %g, want -1", first.Order)
	}
	second, err := blockRepository.CreateBlockForAuthorizedPage(pageID, &mongomodels.BlockMongo{URL: "https://second.example"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Order != -2 {
		t.Fatalf("second block order = %g, want -2", second.Order)
	}
	blocks, err := blockRepository.GetBlocksByPage(pageID.Hex(), ownerID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].Id != second.Id || blocks[1].Id != first.Id {
		t.Fatalf("blocks are not listed newest first: %+v", blocks)
	}
}

func TestConcurrentBlockOrderAllocationsAreUnique(t *testing.T) {
	blockRepository, _, _, _, pageID := blockRepositoryFixture(t, "concurrent sequence")
	const count = 32

	orders := make(chan float64, count)
	errors := make(chan error, count)
	var workers sync.WaitGroup
	for i := range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			block, err := blockRepository.CreateBlockForAuthorizedPage(pageID, &mongomodels.BlockMongo{URL: fmt.Sprintf("https://example.com/%d", i)})
			if err != nil {
				errors <- err
				return
			}
			orders <- block.Order
		}()
	}
	workers.Wait()
	close(errors)
	close(orders)
	for err := range errors {
		t.Fatal(err)
	}

	seen := make(map[float64]bool, count)
	for order := range orders {
		if seen[order] {
			t.Fatalf("duplicate allocated order %g", order)
		}
		seen[order] = true
	}
	for order := -1.0; order >= -count; order-- {
		if !seen[order] {
			t.Fatalf("missing allocated order %g: %+v", order, seen)
		}
	}
}

func TestLegacyPositiveBlockDoesNotAffectSequenceAllocation(t *testing.T) {
	blockRepository, client, ctx, _, pageID := blockRepositoryFixture(t, "legacy order")
	const oldOrder = 1e18
	_, err := client.Database("Auth").Collection("Block").InsertOne(ctx, mongomodels.BlockMongo{PageId: pageID, URL: "https://old.example", Order: oldOrder})
	if err != nil {
		t.Fatal(err)
	}
	created, err := blockRepository.CreateBlockForAuthorizedPage(pageID, &mongomodels.BlockMongo{URL: "https://new.example"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Order != -1 || !(created.Order < oldOrder) {
		t.Fatalf("created order = %g, want -1 before legacy order", created.Order)
	}
}

func TestManualLowerReorderSynchronizesNextAllocation(t *testing.T) {
	blockRepository, _, _, ownerID, pageID := blockRepositoryFixture(t, "manual reorder")
	block, err := blockRepository.CreateBlockForAuthorizedPage(pageID, &mongomodels.BlockMongo{URL: "https://reordered.example"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := blockRepository.UpdateBlock(block.Id.Hex(), ownerID.Hex(), bson.M{"order": float64(-100)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Order != -100 {
		t.Fatalf("updated order = %g, want -100", updated.Order)
	}
	created, err := blockRepository.CreateBlockForAuthorizedPage(pageID, &mongomodels.BlockMongo{URL: "https://new.example"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Order != -101 {
		t.Fatalf("order after manual reorder = %g, want -101", created.Order)
	}
}
