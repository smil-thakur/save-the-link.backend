package workspacecontroller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	workspacecontroller "github.com/smil-thakur/save-the-link/internals/controllers/workspace_controller"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"github.com/smil-thakur/save-the-link/internals/routes"
	workspaceservice "github.com/smil-thakur/save-the-link/internals/services/workspace_service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestWorkspaceImportPrevalidationAndRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	blockRepository := repository.NewBlockRepository(client, ctx, pageRepository)
	controller := workspacecontroller.NewWorkspaceController(workspaceservice.NewWorkspaceService(pageRepository, blockRepository))
	router := gin.New()
	protected := router.Group("/", func(c *gin.Context) { c.Set("userId", ownerID.Hex()) })
	routes.WorkspaceRoutes(protected, controller)
	t.Cleanup(func() {
		var pages []mongomodels.PageMongo
		cursor, _ := client.Database("Auth").Collection("Page").Find(ctx, bson.M{"ownerId": ownerID})
		if cursor != nil {
			_ = cursor.All(ctx, &pages)
		}
		ids := make([]bson.ObjectID, len(pages))
		for i := range pages {
			ids[i] = pages[i].Id
		}
		_, _ = client.Database("Auth").Collection("Block").DeleteMany(ctx, bson.M{"pageId": bson.M{"$in": ids}})
		_, _ = client.Database("Auth").Collection("Page").DeleteMany(ctx, bson.M{"ownerId": ownerID})
	})

	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}

	cycle := `{"version":1,"pages":[{"sourceId":"a","parentSourceId":"b","title":"A","layout":"grid","order":1,"blocks":[]},{"sourceId":"b","parentSourceId":"a","title":"B","layout":"grid","order":2,"blocks":[]}]}`
	response := request(http.MethodPost, "/workspace/import", cycle)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected cycle rejection, got %d: %s", response.Code, response.Body.String())
	}
	count, err := client.Database("Auth").Collection("Page").CountDocuments(ctx, bson.M{"ownerId": ownerID})
	if err != nil || count != 0 {
		t.Fatalf("prevalidation wrote data: count=%d err=%v", count, err)
	}
	invalidArchives := []string{
		`{"version":1,"pages":[{"sourceId":"a","title":"A","layout":"list","order":1,"blocks":[]}]}`,
		`{"version":1,"pages":[{"sourceId":"a","title":"A","layout":"grid","order":1,"blocks":[{"url":"https://example.com","fetchStatus":"pending","order":1}]}]}`,
		`{"version":1,"pages":[{"sourceId":"a","title":"A","layout":"grid","order":1,"blocks":[{"url":"https://example.com","description":"` + strings.Repeat("x", 10001) + `","fetchStatus":"ok","order":1}]}]}`,
	}
	for _, invalidArchive := range invalidArchives {
		response = request(http.MethodPost, "/workspace/import", invalidArchive)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("expected archive validation failure, got %d: %s", response.Code, response.Body.String())
		}
	}

	richDescription := `<p onclick="steal()">Keep <strong>bold</strong> and <em>emphasis</em><br>` +
		`<a href="https://example.com" target="_blank" rel="noopener">safe link</a>` +
		`<a href="javascript:alert(1)">unsafe link</a></p><ul><li>item</li></ul><script>alert(1)</script>`
	rootSourceID := "root"
	icon := "C"
	archive := dto.WorkspaceArchiveDTO{Version: 1, Pages: []dto.WorkspacePageDTO{
		{
			SourceID: "child", ParentSourceID: &rootSourceID, Title: "Child", Icon: &icon, Layout: "freeform", Order: 20,
			Blocks: []dto.WorkspaceBlockDTO{
				{
					URL: "https://example.com", Title: "Example", Description: richDescription,
					CoverImage: "cover", Favicon: "icon", SiteName: "site", FetchStatus: "ok", Tags: []string{"tag"}, Order: 1e18,
				},
				{URL: "https://lower.example", Title: "Lower", FetchStatus: "failed", Tags: []string{}, Order: -10},
			},
		},
		{SourceID: rootSourceID, Title: "Root", Layout: "grid", Order: 10, Blocks: []dto.WorkspaceBlockDTO{}},
	}}
	archiveJSON, err := json.Marshal(archive)
	if err != nil {
		t.Fatal(err)
	}
	response = request(http.MethodPost, "/workspace/import", string(archiveJSON))
	if response.Code != http.StatusCreated {
		t.Fatalf("expected import success, got %d: %s", response.Code, response.Body.String())
	}

	pages, err := pageRepository.GetPagesByOwner(ownerID.Hex())
	if err != nil || len(pages) != 2 {
		t.Fatalf("expected two imported pages, got %d err=%v", len(pages), err)
	}
	byTitle := make(map[string]mongomodels.PageMongo)
	for _, page := range pages {
		byTitle[page.Title] = page
		if page.Visibility != "private" || page.Collaboration != "none" || page.Slug != nil || len(page.CollaboratorIds) != 0 {
			t.Fatalf("imported page is not private/unshared: %+v", page)
		}
	}
	if byTitle["Child"].ParentPageId == nil || *byTitle["Child"].ParentPageId != byTitle["Root"].Id {
		t.Fatalf("hierarchy was not preserved")
	}
	blocks, err := blockRepository.GetBlocksByPageId(byTitle["Child"].Id.Hex())
	if err != nil || len(blocks) != 2 {
		t.Fatalf("expected two imported blocks, got %d err=%v", len(blocks), err)
	}
	if blocks[0].Title != "Lower" || blocks[0].Order != -10 || blocks[1].Title != "Example" || blocks[1].Order != 1e18 || blocks[1].FetchStatus != "ok" || len(blocks[1].Tags) != 1 {
		t.Fatalf("block metadata/order was not preserved: %+v", blocks)
	}
	createdAfterImport, err := blockRepository.CreateBlockForAuthorizedPage(byTitle["Child"].Id, &mongomodels.BlockMongo{URL: "https://new.example", Title: "New", FetchStatus: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if createdAfterImport.Order != -11 || !(createdAfterImport.Order < blocks[0].Order) {
		t.Fatalf("new block order = %g, want -11 before imported order %g", createdAfterImport.Order, blocks[0].Order)
	}
	for _, allowed := range []string{"<p>", "<strong>bold</strong>", "<em>emphasis</em>", "<br>", `<a href="https://example.com" target="_blank" rel="noopener">safe link</a>`, "<ul><li>item</li></ul>"} {
		if !strings.Contains(blocks[1].Description, allowed) {
			t.Fatalf("allowed formatting %q was not preserved: %s", allowed, blocks[1].Description)
		}
	}
	for _, unsafe := range []string{"<script", "onclick", "javascript:"} {
		if strings.Contains(strings.ToLower(blocks[1].Description), unsafe) {
			t.Fatalf("unsafe markup %q remained: %s", unsafe, blocks[1].Description)
		}
	}

	response = request(http.MethodGet, "/workspace/export", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected export success, got %d: %s", response.Code, response.Body.String())
	}
	var exported dto.WorkspaceArchiveDTO
	if err := json.Unmarshal(response.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Version != 1 || len(exported.Pages) != 2 {
		t.Fatalf("unexpected export: %s", response.Body.String())
	}
	var exportedDescription string
	for _, page := range exported.Pages {
		if page.Title == "Child" && len(page.Blocks) == 3 {
			for _, block := range page.Blocks {
				if block.Title == "Example" {
					exportedDescription = block.Description
				}
			}
		}
	}
	if exportedDescription != blocks[1].Description {
		t.Fatalf("export changed sanitized rich-text formatting: got %q want %q", exportedDescription, blocks[1].Description)
	}

	failureURL := "https://rollback.example"
	indexName := "workspace_rollback_" + bson.NewObjectID().Hex()
	_, err = client.Database("Auth").Collection("Block").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "url", Value: 1}},
		Options: options.Index().SetName(indexName).SetUnique(true).SetPartialFilterExpression(
			bson.M{"url": failureURL},
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Database("Auth").Collection("Block").Indexes().DropOne(ctx, indexName) })
	rollbackArchive := `{"version":1,"pages":[{"sourceId":"rollback","title":"Rollback","layout":"grid","order":1,"blocks":[` +
		`{"url":"` + failureURL + `","fetchStatus":"ok","order":1},` +
		`{"url":"` + failureURL + `","fetchStatus":"failed","order":2}` +
		`]}]}`
	response = request(http.MethodPost, "/workspace/import", rollbackArchive)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected induced write failure, got %d: %s", response.Code, response.Body.String())
	}
	count, err = client.Database("Auth").Collection("Page").CountDocuments(ctx, bson.M{"ownerId": ownerID})
	if err != nil || count != 2 {
		t.Fatalf("failed import left pages behind: count=%d err=%v", count, err)
	}
	blockCount, err := client.Database("Auth").Collection("Block").CountDocuments(ctx, bson.M{"url": failureURL})
	if err != nil || blockCount != 0 {
		t.Fatalf("failed import left blocks behind: count=%d err=%v", blockCount, err)
	}
}
