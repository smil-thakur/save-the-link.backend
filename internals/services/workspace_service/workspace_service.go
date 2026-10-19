package workspaceservice

import (
	"math"
	"strings"
	"time"

	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	htmlsanitizer "github.com/smil-thakur/save-the-link/internals/html_sanitizer"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	archiveVersion      = 1
	maxArchivePages     = 500
	maxArchiveBlocks    = 10000
	maxDescriptionBytes = 10000
)

type WorkspaceService struct {
	pageRepository  *repository.PageRepository
	blockRepository *repository.BlockRepository
}

func NewWorkspaceService(pageRepository *repository.PageRepository, blockRepository *repository.BlockRepository) *WorkspaceService {
	return &WorkspaceService{pageRepository: pageRepository, blockRepository: blockRepository}
}

func (s *WorkspaceService) Export(ownerId string) (*dto.WorkspaceArchiveDTO, error) {
	pages, err := s.pageRepository.GetPagesByOwner(ownerId)
	if err != nil {
		return nil, err
	}
	archive := &dto.WorkspaceArchiveDTO{Version: archiveVersion, ExportedAt: time.Now().UTC(), Pages: make([]dto.WorkspacePageDTO, len(pages))}
	for i, page := range pages {
		var parentSourceID *string
		if page.ParentPageId != nil {
			id := page.ParentPageId.Hex()
			parentSourceID = &id
		}
		blocks, err := s.blockRepository.GetBlocksByPageId(page.Id.Hex())
		if err != nil {
			return nil, err
		}
		exportedBlocks := make([]dto.WorkspaceBlockDTO, len(blocks))
		for j, block := range blocks {
			exportedBlocks[j] = dto.WorkspaceBlockDTO{
				URL: block.URL, Title: block.Title, Description: block.Description,
				CoverImage: block.CoverImage, Favicon: block.Favicon, SiteName: block.SiteName,
				FetchStatus: block.FetchStatus, Tags: block.Tags, Order: block.Order,
			}
		}
		archive.Pages[i] = dto.WorkspacePageDTO{
			SourceID: page.Id.Hex(), ParentSourceID: parentSourceID, Title: page.Title,
			Icon: page.Icon, Layout: page.Layout, Order: page.Order, Blocks: exportedBlocks,
		}
	}
	return archive, nil
}

func (s *WorkspaceService) Import(ownerId string, archive dto.WorkspaceArchiveDTO) (*dto.WorkspaceImportResponseDTO, error) {
	if _, err := bson.ObjectIDFromHex(ownerId); err != nil {
		return nil, customerrors.ErrorInvalidWorkspace
	}
	if err := validateArchive(archive); err != nil {
		return nil, err
	}

	ownerObjectId, _ := bson.ObjectIDFromHex(ownerId)
	pageIds := make(map[string]bson.ObjectID, len(archive.Pages))
	createdPageIds := make([]bson.ObjectID, len(archive.Pages))
	for i, source := range archive.Pages {
		id := bson.NewObjectID()
		pageIds[source.SourceID] = id
		createdPageIds[i] = id
	}

	now := time.Now()
	pages := make([]mongomodels.PageMongo, len(archive.Pages))
	blocks := make([]mongomodels.BlockMongo, 0)
	for i, source := range archive.Pages {
		var parentID *bson.ObjectID
		if source.ParentSourceID != nil {
			id := pageIds[*source.ParentSourceID]
			parentID = &id
		}
		blockOrderSequence := 0.0
		for _, sourceBlock := range source.Blocks {
			if sourceBlock.Order < blockOrderSequence {
				blockOrderSequence = sourceBlock.Order
			}
		}
		pages[i] = mongomodels.PageMongo{
			Id: createdPageIds[i], OwnerId: ownerObjectId, ParentPageId: parentID,
			Title: strings.TrimSpace(source.Title), Icon: source.Icon, Layout: source.Layout,
			Visibility: "private", Collaboration: "none", Order: source.Order, BlockOrderSequence: blockOrderSequence,
			CreatedAt: now, UpdatedAt: now,
		}
		for _, sourceBlock := range source.Blocks {
			blocks = append(blocks, mongomodels.BlockMongo{
				Id: bson.NewObjectID(), PageId: createdPageIds[i], URL: strings.TrimSpace(sourceBlock.URL),
				Title: sourceBlock.Title, Description: htmlsanitizer.SanitizeRichText(sourceBlock.Description),
				CoverImage: sourceBlock.CoverImage, Favicon: sourceBlock.Favicon, SiteName: sourceBlock.SiteName,
				FetchStatus: sourceBlock.FetchStatus, Tags: sourceBlock.Tags, Order: sourceBlock.Order,
				CreatedAt: now, UpdatedAt: now,
			})
		}
	}

	cleanup := func() {
		_ = s.blockRepository.DeleteBlocksForPages(createdPageIds)
		_ = s.pageRepository.DeletePagesByIds(createdPageIds)
	}
	if err := s.pageRepository.InsertImportedPages(pages); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.blockRepository.InsertImportedBlocks(blocks); err != nil {
		cleanup()
		return nil, err
	}
	return &dto.WorkspaceImportResponseDTO{PagesCreated: len(pages), BlocksCreated: len(blocks)}, nil
}

func validateArchive(archive dto.WorkspaceArchiveDTO) error {
	if archive.Version != archiveVersion || len(archive.Pages) > maxArchivePages {
		return customerrors.ErrorInvalidWorkspace
	}
	pages := make(map[string]dto.WorkspacePageDTO, len(archive.Pages))
	blockCount := 0
	for _, page := range archive.Pages {
		if page.SourceID == "" || len(page.SourceID) > 128 || strings.TrimSpace(page.Title) == "" || len(page.Title) > 500 || (page.Layout != "grid" && page.Layout != "freeform") || math.IsNaN(page.Order) || math.IsInf(page.Order, 0) {
			return customerrors.ErrorInvalidWorkspace
		}
		if _, exists := pages[page.SourceID]; exists {
			return customerrors.ErrorInvalidWorkspace
		}
		pages[page.SourceID] = page
		blockCount += len(page.Blocks)
		if blockCount > maxArchiveBlocks {
			return customerrors.ErrorInvalidWorkspace
		}
		for _, block := range page.Blocks {
			if strings.TrimSpace(block.URL) == "" || len(block.URL) > 4096 || len(block.Description) > maxDescriptionBytes || (block.FetchStatus != "ok" && block.FetchStatus != "failed") || len(block.Tags) > 100 || math.IsNaN(block.Order) || math.IsInf(block.Order, 0) {
				return customerrors.ErrorInvalidWorkspace
			}
		}
	}
	for _, page := range archive.Pages {
		if page.ParentSourceID != nil {
			if *page.ParentSourceID == page.SourceID {
				return customerrors.ErrorInvalidWorkspace
			}
			if _, exists := pages[*page.ParentSourceID]; !exists {
				return customerrors.ErrorInvalidWorkspace
			}
		}
	}
	state := make(map[string]uint8, len(pages))
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 {
			return false
		}
		if state[id] == 2 {
			return true
		}
		state[id] = 1
		if parent := pages[id].ParentSourceID; parent != nil && !visit(*parent) {
			return false
		}
		state[id] = 2
		return true
	}
	for id := range pages {
		if !visit(id) {
			return customerrors.ErrorInvalidWorkspace
		}
	}
	return nil
}
