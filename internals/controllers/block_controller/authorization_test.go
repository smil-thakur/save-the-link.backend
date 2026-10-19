package blockcontroller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	blockcontroller "github.com/smil-thakur/save-the-link/internals/controllers/block_controller"
	pagecontroller "github.com/smil-thakur/save-the-link/internals/controllers/page_controller"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	"github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"github.com/smil-thakur/save-the-link/internals/routes"
	blockservice "github.com/smil-thakur/save-the-link/internals/services/block_service"
	metadataservice "github.com/smil-thakur/save-the-link/internals/services/metadata_service"
	pageservice "github.com/smil-thakur/save-the-link/internals/services/page_service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func integrationClient(t *testing.T) (*mongo.Client, context.Context) {
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
	return client, ctx
}

func TestBlockCreationAuthorizationMatrixAndOwnerOnlyPageMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, ctx := integrationClient(t)
	ownerID, collaboratorID, strangerID := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	now := time.Now()
	pages := []mongomodels.PageMongo{
		{Id: bson.NewObjectID(), OwnerId: ownerID, Title: "private", Layout: "grid", Visibility: "private", Collaboration: "none", CreatedAt: now, UpdatedAt: now},
		{Id: bson.NewObjectID(), OwnerId: ownerID, Title: "view", Layout: "grid", Visibility: "public", Collaboration: "view", CreatedAt: now, UpdatedAt: now},
		{Id: bson.NewObjectID(), OwnerId: ownerID, Title: "edit", Layout: "grid", Visibility: "public", Collaboration: "edit", CreatedAt: now, UpdatedAt: now},
		{Id: bson.NewObjectID(), OwnerId: ownerID, Title: "invite", Layout: "grid", Visibility: "public", Collaboration: "invite", CollaboratorIds: []bson.ObjectID{collaboratorID}, CreatedAt: now, UpdatedAt: now},
	}
	docs := make([]any, len(pages))
	pageIDs := make([]bson.ObjectID, len(pages))
	for i := range pages {
		docs[i], pageIDs[i] = pages[i], pages[i].Id
	}
	if _, err := client.Database("Auth").Collection("Page").InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": bson.M{"$in": pageIDs}})
		_, _ = client.Database("Auth").Collection("Page").DeleteMany(ctx, bson.M{"_id": bson.M{"$in": pageIDs}})
	})

	pageRepository := repository.NewPageRepository(client, ctx)
	blockRepository := repository.NewBlockRepository(client, ctx, pageRepository)
	fetcher := func(context.Context, string) (*metadataservice.Metadata, error) {
		return &metadataservice.Metadata{Title: "test"}, nil
	}
	blockController := blockcontroller.NewBlockController(blockservice.NewBlockServiceWithMetadataFetcher(blockRepository, fetcher), nil)
	pageController := pagecontroller.NewPageController(pageservice.NewPageService(pageRepository, blockRepository, nil))
	router := gin.New()
	protected := router.Group("/", func(c *gin.Context) {
		c.Set("userId", c.GetHeader("X-Test-User"))
		c.Next()
	})
	routes.BlockRoutes(protected, blockController)
	routes.PageRoutes(protected, pageController)

	tests := []struct {
		name      string
		page      mongomodels.PageMongo
		requester bson.ObjectID
		want      int
	}{
		{"owner private", pages[0], ownerID, http.StatusCreated},
		{"stranger private", pages[0], strangerID, http.StatusForbidden},
		{"stranger view", pages[1], strangerID, http.StatusForbidden},
		{"stranger edit", pages[2], strangerID, http.StatusCreated},
		{"invite collaborator", pages[3], collaboratorID, http.StatusCreated},
		{"invite stranger", pages[3], strangerID, http.StatusForbidden},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/pages/"+test.page.Id.Hex()+"/blocks", bytes.NewBufferString(fmt.Sprintf(`{"url":"https://example.com/%d"}`, i)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User", test.requester.Hex())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != test.want {
				t.Fatalf("expected %d, got %d: %s", test.want, response.Code, response.Body.String())
			}
		})
	}

	for _, test := range []struct {
		requester bson.ObjectID
		want      int
	}{{strangerID, http.StatusForbidden}, {ownerID, http.StatusOK}} {
		req := httptest.NewRequest(http.MethodPatch, "/pages/"+pages[2].Id.Hex(), bytes.NewBufferString(`{"title":"changed"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User", test.requester.Hex())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != test.want {
			t.Fatalf("owner mutation boundary: expected %d, got %d: %s", test.want, response.Code, response.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPatch, "/pages/"+pages[3].Id.Hex()+"/collaborators", bytes.NewBufferString(`{"emails":["registered@example.com"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", strangerID.Hex())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("collaborator update must authorize before user lookup, got %d: %s", response.Code, response.Body.String())
	}
}

func TestBulkImportReturnsPartialResultsWithoutNetwork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, ctx := integrationClient(t)
	ownerID := bson.NewObjectID()
	pageRepository := repository.NewPageRepository(client, ctx)
	page, err := pageRepository.CreatePage(ownerID.Hex(), "bulk", nil)
	if err != nil {
		t.Fatal(err)
	}
	blockRepository := repository.NewBlockRepository(client, ctx, pageRepository)
	if _, err := blockRepository.CreateBlockForAuthorizedPage(page.Id, &mongomodels.BlockMongo{URL: "http://example.com", Title: "existing"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": page.Id})
		_, _ = client.Database("Auth").Collection("Page").DeleteOne(ctx, bson.M{"_id": page.Id})
	})
	var fetchCount atomic.Int32
	fetcher := func(_ context.Context, rawURL string) (*metadataservice.Metadata, error) {
		fetchCount.Add(1)
		if rawURL == "https://metadata-fails.example" {
			return nil, errors.New("offline")
		}
		return &metadataservice.Metadata{Title: "fetched"}, nil
	}
	controller := blockcontroller.NewBlockController(blockservice.NewBlockServiceWithMetadataFetcher(blockRepository, fetcher), nil)
	router := gin.New()
	protected := router.Group("/", func(c *gin.Context) { c.Set("userId", ownerID.Hex()) })
	routes.BlockRoutes(protected, controller)
	body := fmt.Sprintf(`{"urls":["HTTP://EXAMPLE.COM:80/#x","https://new.example","https://new.example/","https://metadata-fails.example","","ftp://example.com","https://example.com/%s"]}`, strings.Repeat("x", 4096))
	req := httptest.NewRequest(http.MethodPost, "/pages/"+page.Id.Hex()+"/blocks/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		Created []struct {
			FetchStatus string `json:"fetchStatus"`
		} `json:"created"`
		Warnings []struct {
			Code string `json:"code"`
		} `json:"warnings"`
		Failed []any `json:"failed"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 2 || len(result.Warnings) != 2 || len(result.Failed) != 3 {
		t.Fatalf("unexpected partial result: %s", response.Body.String())
	}
	if result.Warnings[0].Code != "duplicate_url" || result.Warnings[1].Code != "duplicate_url" {
		t.Fatalf("duplicate warning contract changed: %s", response.Body.String())
	}
	if result.Created[1].FetchStatus != "failed" {
		t.Fatalf("metadata failure should still save: %s", response.Body.String())
	}
	if fetchCount.Load() != 2 {
		t.Fatalf("invalid and duplicate URLs must not fetch metadata; got %d calls", fetchCount.Load())
	}
	blocks, err := blockRepository.GetBlocksByPageId(page.Id.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 3 || blocks[0].URL != "https://new.example" || blocks[1].URL != "https://metadata-fails.example" || blocks[2].URL != "http://example.com" {
		t.Fatalf("bulk-created links must all precede the old link: %+v", blocks)
	}
	if !(blocks[0].Order < blocks[1].Order && blocks[1].Order < blocks[2].Order) {
		t.Fatalf("bulk-created link orders are not strict predecessors: %+v", blocks)
	}
}

func TestBulkImportBoundsConcurrencyPreservesOrderAndHonorsCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, ctx := integrationClient(t)
	ownerID := bson.NewObjectID()
	pageRepository := repository.NewPageRepository(client, ctx)
	page, err := pageRepository.CreatePage(ownerID.Hex(), "concurrent bulk", nil)
	if err != nil {
		t.Fatal(err)
	}
	blockRepository := repository.NewBlockRepository(client, ctx, pageRepository)
	t.Cleanup(func() {
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": page.Id})
		_, _ = client.Database("Auth").Collection("Page").DeleteOne(ctx, bson.M{"_id": page.Id})
	})

	var active, maximum atomic.Int32
	fetcher := func(_ context.Context, rawURL string) (*metadataservice.Metadata, error) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		active.Add(-1)
		return &metadataservice.Metadata{Title: rawURL}, nil
	}
	controller := blockcontroller.NewBlockController(blockservice.NewBlockServiceWithMetadataFetcher(blockRepository, fetcher), nil)
	router := gin.New()
	protected := router.Group("/", func(c *gin.Context) { c.Set("userId", ownerID.Hex()) })
	routes.BlockRoutes(protected, controller)
	urls := make([]string, 12)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://example.com/%02d", i)
	}
	payload, err := json.Marshal(map[string]any{"urls": urls})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/pages/"+page.Id.Hex()+"/blocks/import", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var imported struct {
		Created []struct {
			URL string `json:"url"`
		} `json:"created"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() < 2 || maximum.Load() > 4 {
		t.Fatalf("expected bounded concurrent fetching, max concurrency was %d", maximum.Load())
	}
	if len(imported.Created) != len(urls) {
		t.Fatalf("expected %d created links, got %d", len(urls), len(imported.Created))
	}
	for i := range urls {
		if imported.Created[i].URL != urls[i] {
			t.Fatalf("created results lost input order at %d: got %q want %q", i, imported.Created[i].URL, urls[i])
		}
	}

	cancelingFetcher := func(ctx context.Context, _ string) (*metadataservice.Metadata, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	cancelCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result, err := blockservice.NewBlockServiceWithMetadataFetcher(blockRepository, cancelingFetcher).ImportURLs(cancelCtx, page.Id.Hex(), ownerID.Hex(), []string{"https://cancel.example"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 0 || len(result.Failed) != 1 {
		t.Fatalf("canceled import should not write: %+v", result)
	}
	count, err := client.Database("Auth").Collection("Block").CountDocuments(ctx, bson.M{"pageId": page.Id})
	if err != nil || count != int64(len(urls)) {
		t.Fatalf("canceled import changed stored blocks: count=%d err=%v", count, err)
	}
}

func TestUpdateBlockSanitizesRichTextDescription(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, ctx := integrationClient(t)
	ownerID := bson.NewObjectID()
	pageRepository := repository.NewPageRepository(client, ctx)
	page, err := pageRepository.CreatePage(ownerID.Hex(), "sanitize update", nil)
	if err != nil {
		t.Fatal(err)
	}
	blockRepository := repository.NewBlockRepository(client, ctx, pageRepository)
	block, err := blockRepository.CreateBlockForAuthorizedPage(page.Id, &mongomodels.BlockMongo{URL: "https://example.com", Title: "example", FetchStatus: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": page.Id})
		_, _ = client.Database("Auth").Collection("Page").DeleteOne(ctx, bson.M{"_id": page.Id})
	})

	controller := blockcontroller.NewBlockController(blockservice.NewBlockService(blockRepository), nil)
	router := gin.New()
	protected := router.Group("/", func(c *gin.Context) { c.Set("userId", ownerID.Hex()) })
	routes.BlockRoutes(protected, controller)
	description := `<p onmouseover="steal()"><strong>keep</strong> <a href="https://example.com">safe</a>` +
		` <a href="javascript:alert(1)">unsafe</a></p><blockquote>quote</blockquote><script>alert(1)</script>`
	payload, err := json.Marshal(map[string]string{"description": description})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/blocks/"+block.Id.Hex(), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var updated struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	for _, allowed := range []string{"<p>", "<strong>keep</strong>", `<a href="https://example.com">safe</a>`, "<blockquote>quote</blockquote>"} {
		if !strings.Contains(updated.Description, allowed) {
			t.Fatalf("allowed formatting %q was removed: %s", allowed, updated.Description)
		}
	}
	for _, unsafe := range []string{"<script", "onmouseover", "javascript:"} {
		if strings.Contains(strings.ToLower(updated.Description), unsafe) {
			t.Fatalf("unsafe markup %q was persisted: %s", unsafe, updated.Description)
		}
	}
}

func TestBulkImportReauthorizesAfterMetadataFetch(t *testing.T) {
	client, ctx := integrationClient(t)
	ownerID, collaboratorID := bson.NewObjectID(), bson.NewObjectID()
	pageRepository := repository.NewPageRepository(client, ctx)
	page, err := pageRepository.CreatePage(ownerID.Hex(), "revoked bulk", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Database("Auth").Collection("Page").UpdateOne(ctx, bson.M{"_id": page.Id}, bson.M{"$set": bson.M{"visibility": "public", "collaboration": "edit"}})
	if err != nil {
		t.Fatal(err)
	}
	blockRepository := repository.NewBlockRepository(client, ctx, pageRepository)
	t.Cleanup(func() {
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": page.Id})
		_, _ = client.Database("Auth").Collection("Page").DeleteOne(ctx, bson.M{"_id": page.Id})
	})
	fetcher := func(context.Context, string) (*metadataservice.Metadata, error) {
		_, err := client.Database("Auth").Collection("Page").UpdateOne(ctx, bson.M{"_id": page.Id}, bson.M{"$set": bson.M{"collaboration": "view"}})
		if err != nil {
			return nil, err
		}
		return &metadataservice.Metadata{Title: "fetched"}, nil
	}

	result, err := blockservice.NewBlockServiceWithMetadataFetcher(blockRepository, fetcher).ImportURLs(context.Background(), page.Id.Hex(), collaboratorID.Hex(), []string{"https://example.com/revoked"})
	if result != nil || !errors.Is(err, customerrors.ErrorForbidden) {
		t.Fatalf("expected revoked access to stop import, result=%+v err=%v", result, err)
	}
	count, err := client.Database("Auth").Collection("Block").CountDocuments(ctx, bson.M{"pageId": page.Id})
	if err != nil || count != 0 {
		t.Fatalf("revoked collaborator wrote blocks: count=%d err=%v", count, err)
	}
}
