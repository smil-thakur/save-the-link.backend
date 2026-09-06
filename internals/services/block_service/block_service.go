package blockservice

import (
	"context"
	"strings"

	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	metadataservice "github.com/smil-thakur/save-the-link/internals/services/metadata_service"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type BlockService struct {
	blockRepository *repository.BlockRepository
}

func NewBlockService(blockRepository *repository.BlockRepository) *BlockService {
	return &BlockService{
		blockRepository: blockRepository,
	}
}

func (s *BlockService) CreateBlock(ctx context.Context, pageId string, ownerId string, rawURL string) (*mongomodels.BlockMongo, error) {
	url := strings.TrimSpace(rawURL)

	if url == "" {
		return nil, customerrors.ErrorInvalidBlock
	}

	block := &mongomodels.BlockMongo{
		URL:  url,
		Tags: []string{},
	}

	meta, err := metadataservice.FetchMetadata(ctx, url)

	if err != nil {
		block.FetchStatus = "failed"
		block.Title = url
	} else {
		block.FetchStatus = "ok"
		block.Title = meta.Title
		if block.Title == "" {
			block.Title = url
		}
		block.Description = meta.Description
		block.CoverImage = meta.Image
		block.Favicon = meta.Favicon
		block.SiteName = meta.SiteName
	}

	return s.blockRepository.CreateBlock(pageId, ownerId, block)
}

func (s *BlockService) GetBlocks(pageId string, ownerId string) ([]mongomodels.BlockMongo, error) {
	return s.blockRepository.GetBlocksByPage(pageId, ownerId)
}

// GetPublicBlocks lists a page's blocks with no ownership check — callers must have
// already verified the page is public (e.g. via a slug lookup).
func (s *BlockService) GetPublicBlocks(pageId string) ([]mongomodels.BlockMongo, error) {
	return s.blockRepository.GetBlocksByPageId(pageId)
}

func (s *BlockService) UpdateBlock(blockId string, ownerId string, body dto.UpdateBlockDTO) (*mongomodels.BlockMongo, error) {
	updates := bson.M{}

	if body.Title != nil {
		title := strings.TrimSpace(*body.Title)

		if title == "" {
			return nil, customerrors.ErrorInvalidBlock
		}

		updates["title"] = title
	}

	if body.Description != nil {
		updates["description"] = *body.Description
	}

	if body.Tags != nil {
		updates["tags"] = *body.Tags
	}

	if body.Order != nil {
		updates["order"] = *body.Order
	}

	return s.blockRepository.UpdateBlock(blockId, ownerId, updates)
}

func (s *BlockService) DeleteBlock(blockId string, ownerId string) error {
	return s.blockRepository.DeleteBlock(blockId, ownerId)
}
