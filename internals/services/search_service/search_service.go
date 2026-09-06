package searchservice

import (
	"strings"

	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
)

type SearchService struct {
	pageRepository  *repository.PageRepository
	blockRepository *repository.BlockRepository
}

func NewSearchService(pageRepository *repository.PageRepository, blockRepository *repository.BlockRepository) *SearchService {
	return &SearchService{
		pageRepository:  pageRepository,
		blockRepository: blockRepository,
	}
}

func (s *SearchService) Search(ownerId string, query string) ([]mongomodels.PageMongo, []mongomodels.BlockMongo, error) {
	trimmed := strings.TrimSpace(query)

	if trimmed == "" {
		return []mongomodels.PageMongo{}, []mongomodels.BlockMongo{}, nil
	}

	pages, err := s.pageRepository.SearchPagesForOwner(ownerId, trimmed)

	if err != nil {
		return nil, nil, err
	}

	blocks, err := s.blockRepository.SearchBlocksForOwner(ownerId, trimmed)

	if err != nil {
		return nil, nil, err
	}

	return pages, blocks, nil
}
