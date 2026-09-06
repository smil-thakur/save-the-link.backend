package repository

import (
	"context"
	"errors"
	"regexp"
	"time"

	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PageRepository struct {
	client *mongo.Client
	ctx    context.Context
}

func NewPageRepository(client *mongo.Client, ctx context.Context) *PageRepository {
	return &PageRepository{
		client: client,
		ctx:    ctx,
	}
}

func (p *PageRepository) collection() *mongo.Collection {
	return p.client.Database("Auth").Collection("Page")
}

func (p *PageRepository) CreatePage(ownerId string, title string, parentPageId *string) (*mongomodels.PageMongo, error) {
	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	var parentObjectId *bson.ObjectID

	if parentPageId != nil {
		id, err := bson.ObjectIDFromHex(*parentPageId)

		if err != nil {
			return nil, customerrors.ErrorInvalidPage
		}

		// A page can only be nested under a page the caller owns.
		_, err = p.GetPageById(*parentPageId, ownerId)

		if err != nil {
			return nil, err
		}

		parentObjectId = &id
	}

	now := time.Now()

	page := &mongomodels.PageMongo{
		OwnerId:       ownerObjectId,
		ParentPageId:  parentObjectId,
		Title:         title,
		Layout:        "grid",
		Visibility:    "private",
		Collaboration: "none",
		Order:         float64(now.UnixNano()),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	result, err := p.collection().InsertOne(p.ctx, page)

	if err != nil {
		return nil, err
	}

	page.Id = result.InsertedID.(bson.ObjectID)

	return page, nil
}

func (p *PageRepository) GetPagesByOwner(ownerId string) ([]mongomodels.PageMongo, error) {
	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	cursor, err := p.collection().Find(
		p.ctx,
		bson.M{"ownerId": ownerObjectId, "deletedAt": bson.M{"$exists": false}},
		options.Find().SetSort(bson.M{"order": 1}),
	)

	if err != nil {
		return nil, err
	}

	var pages []mongomodels.PageMongo

	if err := cursor.All(p.ctx, &pages); err != nil {
		return nil, err
	}

	return pages, nil
}

func (p *PageRepository) SearchPagesForOwner(ownerId string, query string) ([]mongomodels.PageMongo, error) {
	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	pattern := regexp.QuoteMeta(query)

	cursor, err := p.collection().Find(
		p.ctx,
		bson.M{
			"ownerId":   ownerObjectId,
			"deletedAt": bson.M{"$exists": false},
			"title":     bson.M{"$regex": pattern, "$options": "i"},
		},
		options.Find().SetLimit(searchResultLimit).SetSort(bson.M{"updatedAt": -1}),
	)

	if err != nil {
		return nil, err
	}

	var pages []mongomodels.PageMongo

	if err := cursor.All(p.ctx, &pages); err != nil {
		return nil, err
	}

	return pages, nil
}

func (p *PageRepository) GetPageById(id string, ownerId string) (*mongomodels.PageMongo, error) {
	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return nil, customerrors.ErrorPageNotFound
	}

	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	var page mongomodels.PageMongo
	err = p.collection().FindOne(p.ctx, bson.M{
		"_id":       objectId,
		"deletedAt": bson.M{"$exists": false},
	}).Decode(&page)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorPageNotFound
		}
		return nil, err
	}

	if page.OwnerId != ownerObjectId {
		return nil, customerrors.ErrorForbidden
	}

	return &page, nil
}

// GetPageForCollaborator allows access to the page's owner, and additionally to any
// other authenticated caller when the page has been published with edit collaboration.
func (p *PageRepository) GetPageForCollaborator(id string, requesterId string) (*mongomodels.PageMongo, error) {
	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return nil, customerrors.ErrorPageNotFound
	}

	var page mongomodels.PageMongo
	err = p.collection().FindOne(p.ctx, bson.M{
		"_id":       objectId,
		"deletedAt": bson.M{"$exists": false},
	}).Decode(&page)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorPageNotFound
		}
		return nil, err
	}

	requesterObjectId, err := bson.ObjectIDFromHex(requesterId)

	if err == nil && page.OwnerId == requesterObjectId {
		return &page, nil
	}

	if page.Visibility != "public" {
		return nil, customerrors.ErrorForbidden
	}

	if page.Collaboration == "edit" {
		return &page, nil
	}

	if page.Collaboration == "invite" && err == nil {
		for _, collaboratorId := range page.CollaboratorIds {
			if collaboratorId == requesterObjectId {
				return &page, nil
			}
		}
	}

	return nil, customerrors.ErrorForbidden
}

// GetPublicPageBySlug looks up a published page with no ownership check at all —
// used by the unauthenticated public share route.
func (p *PageRepository) GetPublicPageBySlug(slug string) (*mongomodels.PageMongo, error) {
	var page mongomodels.PageMongo
	err := p.collection().FindOne(p.ctx, bson.M{
		"slug":       slug,
		"visibility": "public",
		"deletedAt":  bson.M{"$exists": false},
	}).Decode(&page)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorPageNotFound
		}
		return nil, err
	}

	return &page, nil
}

// GetPageByIdUnscoped fetches a page by id with no ownership filtering at
// all — unlike GetPageById, the caller isn't asserting a specific owner, it's
// asserting some other rule of its own (e.g. bookmarking needs to check both
// that the page is public AND that the requester ISN'T the owner, which
// doesn't fit an owner-scoped lookup either way).
func (p *PageRepository) GetPageByIdUnscoped(id string) (*mongomodels.PageMongo, error) {
	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return nil, customerrors.ErrorPageNotFound
	}

	var page mongomodels.PageMongo
	err = p.collection().FindOne(p.ctx, bson.M{
		"_id":       objectId,
		"deletedAt": bson.M{"$exists": false},
	}).Decode(&page)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorPageNotFound
		}
		return nil, err
	}

	return &page, nil
}

type PublicPageSummary struct {
	Title     string        `bson:"title"`
	Icon      *string       `bson:"icon,omitempty"`
	Slug      string        `bson:"slug"`
	LinkCount int           `bson:"linkCount"`
	UpdatedAt time.Time     `bson:"updatedAt"`
	Id        bson.ObjectID `bson:"_id"`
}

// ListPublicPages lists recently-updated public pages across every user, for
// the "explore" preview shown to logged-out visitors. No ownership scoping —
// these pages are, by definition, already publicly viewable via their slug.
func (p *PageRepository) ListPublicPages(limit int64) ([]PublicPageSummary, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"visibility": "public",
			"deletedAt":  bson.M{"$exists": false},
			"slug":       bson.M{"$exists": true},
		}}},
		{{Key: "$sort", Value: bson.M{"updatedAt": -1}}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "Block",
			"localField":   "_id",
			"foreignField": "pageId",
			"as":           "blocks",
		}}},
		{{Key: "$project", Value: bson.M{
			"title":     1,
			"icon":      1,
			"slug":      1,
			"updatedAt": 1,
			"linkCount": bson.M{"$size": "$blocks"},
		}}},
	}

	cursor, err := p.collection().Aggregate(p.ctx, pipeline)

	if err != nil {
		return nil, err
	}

	var summaries []PublicPageSummary

	if err := cursor.All(p.ctx, &summaries); err != nil {
		return nil, err
	}

	return summaries, nil
}

// GetPageSummariesByIds fetches lightweight public-page summaries for a
// specific set of page ids — used for the bookmarks list. A bookmarked page
// that's since been unpublished, deleted, or made private again is silently
// excluded rather than erroring the whole request.
func (p *PageRepository) GetPageSummariesByIds(ids []bson.ObjectID) ([]PublicPageSummary, error) {
	if len(ids) == 0 {
		return []PublicPageSummary{}, nil
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"_id":        bson.M{"$in": ids},
			"visibility": "public",
			"deletedAt":  bson.M{"$exists": false},
		}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "Block",
			"localField":   "_id",
			"foreignField": "pageId",
			"as":           "blocks",
		}}},
		{{Key: "$project", Value: bson.M{
			"title":     1,
			"icon":      1,
			"slug":      1,
			"updatedAt": 1,
			"linkCount": bson.M{"$size": "$blocks"},
		}}},
	}

	cursor, err := p.collection().Aggregate(p.ctx, pipeline)

	if err != nil {
		return nil, err
	}

	var summaries []PublicPageSummary

	if err := cursor.All(p.ctx, &summaries); err != nil {
		return nil, err
	}

	return summaries, nil
}

func (p *PageRepository) UpdatePage(id string, ownerId string, updates bson.M) (*mongomodels.PageMongo, error) {
	// Ensures the page exists and is owned by the caller before mutating it.
	if _, err := p.GetPageById(id, ownerId); err != nil {
		return nil, err
	}

	objectId, _ := bson.ObjectIDFromHex(id)

	updates["updatedAt"] = time.Now()

	_, err := p.collection().UpdateOne(p.ctx, bson.M{"_id": objectId}, bson.M{"$set": updates})

	if err != nil {
		return nil, err
	}

	return p.GetPageById(id, ownerId)
}

func (p *PageRepository) DeletePage(id string, ownerId string) error {
	page, err := p.GetPageById(id, ownerId)

	if err != nil {
		return err
	}

	descendantIds, err := p.collectDescendantIds(page.Id)

	if err != nil {
		return err
	}

	idsToDelete := append(descendantIds, page.Id)

	_, err = p.collection().UpdateMany(
		p.ctx,
		bson.M{"_id": bson.M{"$in": idsToDelete}},
		bson.M{"$set": bson.M{"deletedAt": time.Now()}},
	)

	return err
}

func (p *PageRepository) GetTrashedPages(ownerId string) ([]mongomodels.PageMongo, error) {
	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	cursor, err := p.collection().Find(
		p.ctx,
		bson.M{"ownerId": ownerObjectId, "deletedAt": bson.M{"$exists": true}},
		options.Find().SetSort(bson.M{"deletedAt": -1}),
	)

	if err != nil {
		return nil, err
	}

	var pages []mongomodels.PageMongo

	if err := cursor.All(p.ctx, &pages); err != nil {
		return nil, err
	}

	return pages, nil
}

func (p *PageRepository) RestorePage(id string, ownerId string) (*mongomodels.PageMongo, error) {
	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return nil, customerrors.ErrorPageNotFound
	}

	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	var page mongomodels.PageMongo
	err = p.collection().FindOne(p.ctx, bson.M{
		"_id":       objectId,
		"deletedAt": bson.M{"$exists": true},
	}).Decode(&page)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorPageNotFound
		}
		return nil, err
	}

	if page.OwnerId != ownerObjectId {
		return nil, customerrors.ErrorForbidden
	}

	// Bring the whole subtree back together, not just this one page.
	descendantIds, err := p.collectAllDescendantIds(page.Id)

	if err != nil {
		return nil, err
	}

	idsToRestore := append(descendantIds, page.Id)

	_, err = p.collection().UpdateMany(
		p.ctx,
		bson.M{"_id": bson.M{"$in": idsToRestore}},
		bson.M{"$unset": bson.M{"deletedAt": ""}, "$set": bson.M{"updatedAt": time.Now()}},
	)

	if err != nil {
		return nil, err
	}

	page.DeletedAt = nil

	return &page, nil
}

// PermanentlyDeletePage removes a trashed page and its entire (also-trashed)
// subtree, along with every block on those pages. This is irreversible.
func (p *PageRepository) PermanentlyDeletePage(id string, ownerId string, blockRepository *BlockRepository) error {
	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return customerrors.ErrorPageNotFound
	}

	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return customerrors.ErrorInvalidPage
	}

	var page mongomodels.PageMongo
	err = p.collection().FindOne(p.ctx, bson.M{
		"_id":       objectId,
		"deletedAt": bson.M{"$exists": true},
	}).Decode(&page)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return customerrors.ErrorPageNotFound
		}
		return err
	}

	if page.OwnerId != ownerObjectId {
		return customerrors.ErrorForbidden
	}

	descendantIds, err := p.collectAllDescendantIds(page.Id)

	if err != nil {
		return err
	}

	idsToDelete := append(descendantIds, page.Id)

	if blockRepository != nil {
		if err := blockRepository.DeleteBlocksForPages(idsToDelete); err != nil {
			return err
		}
	}

	_, err = p.collection().DeleteMany(p.ctx, bson.M{"_id": bson.M{"$in": idsToDelete}})

	return err
}

// DeleteAllForOwner purges every page (and their blocks) belonging to an owner —
// used when a user deletes their account. Irreversible.
func (p *PageRepository) DeleteAllForOwner(ownerId string, blockRepository *BlockRepository) error {
	ownerObjectId, err := bson.ObjectIDFromHex(ownerId)

	if err != nil {
		return customerrors.ErrorInvalidPage
	}

	cursor, err := p.collection().Find(p.ctx, bson.M{"ownerId": ownerObjectId})

	if err != nil {
		return err
	}

	var pages []mongomodels.PageMongo

	if err := cursor.All(p.ctx, &pages); err != nil {
		return err
	}

	if len(pages) == 0 {
		return nil
	}

	ids := make([]bson.ObjectID, len(pages))

	for i, page := range pages {
		ids[i] = page.Id
	}

	if blockRepository != nil {
		if err := blockRepository.DeleteBlocksForPages(ids); err != nil {
			return err
		}
	}

	_, err = p.collection().DeleteMany(p.ctx, bson.M{"_id": bson.M{"$in": ids}})

	return err
}

// collectAllDescendantIds walks the subtree regardless of deletedAt — used when
// permanently purging a page, since its descendants were already soft-deleted too.
func (p *PageRepository) collectAllDescendantIds(rootId bson.ObjectID) ([]bson.ObjectID, error) {
	var descendants []bson.ObjectID
	frontier := []bson.ObjectID{rootId}

	for len(frontier) > 0 {
		cursor, err := p.collection().Find(p.ctx, bson.M{"parentPageId": bson.M{"$in": frontier}})

		if err != nil {
			return nil, err
		}

		var children []mongomodels.PageMongo

		if err := cursor.All(p.ctx, &children); err != nil {
			return nil, err
		}

		frontier = make([]bson.ObjectID, 0, len(children))

		for _, child := range children {
			descendants = append(descendants, child.Id)
			frontier = append(frontier, child.Id)
		}
	}

	return descendants, nil
}

func (p *PageRepository) GetDescendantIds(id string) ([]string, error) {
	objectId, err := bson.ObjectIDFromHex(id)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	ids, err := p.collectDescendantIds(objectId)

	if err != nil {
		return nil, err
	}

	hexIds := make([]string, len(ids))

	for i, oid := range ids {
		hexIds[i] = oid.Hex()
	}

	return hexIds, nil
}

func (p *PageRepository) collectDescendantIds(rootId bson.ObjectID) ([]bson.ObjectID, error) {
	var descendants []bson.ObjectID
	frontier := []bson.ObjectID{rootId}

	for len(frontier) > 0 {
		cursor, err := p.collection().Find(p.ctx, bson.M{
			"parentPageId": bson.M{"$in": frontier},
			"deletedAt":    bson.M{"$exists": false},
		})

		if err != nil {
			return nil, err
		}

		var children []mongomodels.PageMongo

		if err := cursor.All(p.ctx, &children); err != nil {
			return nil, err
		}

		frontier = make([]bson.ObjectID, 0, len(children))

		for _, child := range children {
			descendants = append(descendants, child.Id)
			frontier = append(frontier, child.Id)
		}
	}

	return descendants, nil
}
