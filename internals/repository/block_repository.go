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

const searchResultLimit = 20

type BlockRepository struct {
	client         *mongo.Client
	ctx            context.Context
	pageRepository *PageRepository
}

func NewBlockRepository(client *mongo.Client, ctx context.Context, pageRepository *PageRepository) *BlockRepository {
	return &BlockRepository{
		client:         client,
		ctx:            ctx,
		pageRepository: pageRepository,
	}
}

func (b *BlockRepository) collection() *mongo.Collection {
	return b.client.Database("Auth").Collection("Block")
}

func (b *BlockRepository) CreateBlock(pageId string, ownerId string, block *mongomodels.BlockMongo) (*mongomodels.BlockMongo, error) {
	page, err := b.pageRepository.GetPageForCollaborator(pageId, ownerId)

	if err != nil {
		return nil, err
	}

	now := time.Now()
	block.PageId = page.Id
	block.Order = float64(now.UnixNano())
	block.CreatedAt = now
	block.UpdatedAt = now

	if block.Tags == nil {
		block.Tags = []string{}
	}

	result, err := b.collection().InsertOne(b.ctx, block)

	if err != nil {
		return nil, err
	}

	block.Id = result.InsertedID.(bson.ObjectID)

	return block, nil
}

func (b *BlockRepository) GetBlocksByPage(pageId string, ownerId string) ([]mongomodels.BlockMongo, error) {
	if _, err := b.pageRepository.GetPageForCollaborator(pageId, ownerId); err != nil {
		return nil, err
	}

	return b.getBlocksByPageId(pageId)
}

// GetBlocksByPageId lists a page's blocks with no access check — callers must have
// already verified access to the page (e.g. via a public-slug lookup).
func (b *BlockRepository) GetBlocksByPageId(pageId string) ([]mongomodels.BlockMongo, error) {
	return b.getBlocksByPageId(pageId)
}

func (b *BlockRepository) getBlocksByPageId(pageId string) ([]mongomodels.BlockMongo, error) {
	pageObjectId, err := bson.ObjectIDFromHex(pageId)

	if err != nil {
		return nil, customerrors.ErrorInvalidPage
	}

	cursor, err := b.collection().Find(
		b.ctx,
		bson.M{"pageId": pageObjectId},
		options.Find().SetSort(bson.M{"order": 1}),
	)

	if err != nil {
		return nil, err
	}

	var blocks []mongomodels.BlockMongo

	if err := cursor.All(b.ctx, &blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}

func (b *BlockRepository) getOwnedBlock(blockId string, ownerId string) (*mongomodels.BlockMongo, error) {
	objectId, err := bson.ObjectIDFromHex(blockId)

	if err != nil {
		return nil, customerrors.ErrorBlockNotFound
	}

	var block mongomodels.BlockMongo
	err = b.collection().FindOne(b.ctx, bson.M{"_id": objectId}).Decode(&block)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, customerrors.ErrorBlockNotFound
		}
		return nil, err
	}

	if _, err := b.pageRepository.GetPageForCollaborator(block.PageId.Hex(), ownerId); err != nil {
		return nil, err
	}

	return &block, nil
}

func (b *BlockRepository) UpdateBlock(blockId string, ownerId string, updates bson.M) (*mongomodels.BlockMongo, error) {
	block, err := b.getOwnedBlock(blockId, ownerId)

	if err != nil {
		return nil, err
	}

	updates["updatedAt"] = time.Now()

	_, err = b.collection().UpdateOne(b.ctx, bson.M{"_id": block.Id}, bson.M{"$set": updates})

	if err != nil {
		return nil, err
	}

	return b.getOwnedBlock(blockId, ownerId)
}

func (b *BlockRepository) DeleteBlock(blockId string, ownerId string) error {
	block, err := b.getOwnedBlock(blockId, ownerId)

	if err != nil {
		return err
	}

	_, err = b.collection().DeleteOne(b.ctx, bson.M{"_id": block.Id})

	return err
}

// SearchBlocksForOwner searches title/description/url/tags across every block on
// every page the owner has (their own pages only, not shared-with-them pages).
func (b *BlockRepository) SearchBlocksForOwner(ownerId string, query string) ([]mongomodels.BlockMongo, error) {
	pages, err := b.pageRepository.GetPagesByOwner(ownerId)

	if err != nil {
		return nil, err
	}

	if len(pages) == 0 {
		return []mongomodels.BlockMongo{}, nil
	}

	pageIds := make([]bson.ObjectID, len(pages))

	for i, page := range pages {
		pageIds[i] = page.Id
	}

	pattern := regexp.QuoteMeta(query)
	regex := bson.M{"$regex": pattern, "$options": "i"}

	cursor, err := b.collection().Find(
		b.ctx,
		bson.M{
			"pageId": bson.M{"$in": pageIds},
			"$or": []bson.M{
				{"title": regex},
				{"description": regex},
				{"url": regex},
				{"tags": regex},
			},
		},
		options.Find().SetLimit(searchResultLimit).SetSort(bson.M{"updatedAt": -1}),
	)

	if err != nil {
		return nil, err
	}

	var blocks []mongomodels.BlockMongo

	if err := cursor.All(b.ctx, &blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}

// DeleteBlocksForPages purges every block belonging to any of the given pages,
// with no ownership check — callers must have already verified access (e.g.
// when permanently deleting a page the caller owns).
func (b *BlockRepository) DeleteBlocksForPages(pageIds []bson.ObjectID) error {
	_, err := b.collection().DeleteMany(b.ctx, bson.M{"pageId": bson.M{"$in": pageIds}})

	return err
}
