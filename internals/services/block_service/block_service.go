package blockservice

import (
	"context"
	"errors"
	"math"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	htmlsanitizer "github.com/smil-thakur/save-the-link/internals/html_sanitizer"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	metadataservice "github.com/smil-thakur/save-the-link/internals/services/metadata_service"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	maxImportURLBytes   = 4096
	metadataWorkerCount = 4
	maxImportDuration   = 3 * time.Minute
)

type BlockService struct {
	blockRepository *repository.BlockRepository
	metadataFetcher func(context.Context, string) (*metadataservice.Metadata, error)
}

func NewBlockService(blockRepository *repository.BlockRepository) *BlockService {
	return &BlockService{
		blockRepository: blockRepository,
		metadataFetcher: metadataservice.FetchMetadata,
	}
}

func NewBlockServiceWithMetadataFetcher(blockRepository *repository.BlockRepository, fetcher func(context.Context, string) (*metadataservice.Metadata, error)) *BlockService {
	return &BlockService{blockRepository: blockRepository, metadataFetcher: fetcher}
}

func (s *BlockService) CreateBlock(ctx context.Context, pageId string, ownerId string, rawURL string, allowDuplicate bool) (*mongomodels.BlockMongo, error) {
	blockURL := strings.TrimSpace(rawURL)

	if blockURL == "" {
		return nil, customerrors.ErrorInvalidBlock
	}

	blocks, err := s.blockRepository.GetBlocksByPage(pageId, ownerId)
	if err != nil {
		return nil, err
	}

	if !allowDuplicate {
		// Duplicate detection is intentionally warning-only rather than enforced by
		// a unique index: explicit duplicates are supported, so concurrent creates
		// may both pass this check.
		normalizedURL := normalizeURL(blockURL)
		for _, existing := range blocks {
			if normalizeURL(existing.URL) == normalizedURL {
				return nil, customerrors.ErrorDuplicateURL
			}
		}
	}

	block := s.blockFromURL(ctx, blockURL)
	page, err := s.blockRepository.AuthorizePageForEdit(pageId, ownerId)
	if err != nil {
		return nil, err
	}
	return s.blockRepository.CreateBlockForAuthorizedPageContext(ctx, page.Id, block)
}

func (s *BlockService) blockFromURL(ctx context.Context, blockURL string) *mongomodels.BlockMongo {
	block := &mongomodels.BlockMongo{URL: blockURL, Tags: []string{}}
	meta, err := s.metadataFetcher(ctx, blockURL)

	if err != nil || meta == nil {
		block.FetchStatus = "failed"
		block.Title = blockURL
	} else {
		block.FetchStatus = "ok"
		block.Title = meta.Title
		if block.Title == "" {
			block.Title = blockURL
		}
		block.Description = meta.Description
		block.CoverImage = meta.Image
		block.Favicon = meta.Favicon
		block.SiteName = meta.SiteName
	}

	return block
}

type ImportIssue struct {
	Index   int
	URL     string
	Code    string
	Message string
}

type ImportResult struct {
	Created  []mongomodels.BlockMongo
	Warnings []ImportIssue
	Failed   []ImportIssue
}

type importCandidate struct {
	index int
	url   string
}

type importOutcome struct {
	block *mongomodels.BlockMongo
	issue *ImportIssue
	warn  bool
}

func (s *BlockService) ImportURLs(ctx context.Context, pageId string, requesterId string, urls []string) (*ImportResult, error) {
	workCtx, cancel := context.WithTimeout(ctx, maxImportDuration)
	defer cancel()
	if err := workCtx.Err(); err != nil {
		return nil, err
	}

	page, err := s.blockRepository.AuthorizePageForEdit(pageId, requesterId)
	if err != nil {
		return nil, err
	}
	existing, err := s.blockRepository.GetBlocksByPageId(pageId)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(existing)+len(urls))
	for _, block := range existing {
		seen[normalizeURL(block.URL)] = true
	}
	outcomes := make([]importOutcome, len(urls))
	candidates := make([]importCandidate, 0, len(urls))
	for i, rawURL := range urls {
		blockURL, valid := validateImportURL(rawURL)
		if !valid {
			outcomes[i].issue = &ImportIssue{Index: i, URL: rawURL, Code: "invalid_url", Message: customerrors.ErrorInvalidBlock.Error()}
			continue
		}
		normalized := normalizeURL(blockURL)
		if seen[normalized] {
			outcomes[i].issue = &ImportIssue{Index: i, URL: rawURL, Code: "duplicate_url", Message: customerrors.ErrorDuplicateURL.Error()}
			outcomes[i].warn = true
			continue
		}
		seen[normalized] = true
		candidates = append(candidates, importCandidate{index: i, url: blockURL})
	}

	jobs := make(chan importCandidate, len(candidates))
	var workers sync.WaitGroup
	for range metadataWorkerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for candidate := range jobs {
				block := s.blockFromURL(workCtx, candidate.url)
				if err := workCtx.Err(); err != nil {
					code := "request_canceled"
					if errors.Is(err, context.DeadlineExceeded) {
						code = "import_timeout"
					}
					outcomes[candidate.index].issue = &ImportIssue{Index: candidate.index, URL: candidate.url, Code: code, Message: "link import was canceled"}
					continue
				}
				outcomes[candidate.index].block = block
			}
		}()
	}
	for _, candidate := range candidates {
		jobs <- candidate
	}
	close(jobs)
	workers.Wait()
	if _, err := s.blockRepository.AuthorizePageForEdit(pageId, requesterId); err != nil {
		return nil, err
	}

	creatable := 0
	for _, candidate := range candidates {
		if outcomes[candidate.index].issue == nil {
			creatable++
		}
	}
	nextOrder := 0.0
	if creatable > 0 {
		nextOrder, err = s.blockRepository.ReserveBlockOrdersForAuthorizedPageContext(workCtx, page.Id, creatable)
		if err != nil {
			return nil, err
		}
	}
	for _, candidate := range candidates {
		outcome := &outcomes[candidate.index]
		if outcome.issue != nil {
			continue
		}
		order := nextOrder
		nextOrder++
		if err := workCtx.Err(); err != nil {
			outcome.issue = &ImportIssue{Index: candidate.index, URL: candidate.url, Code: "request_canceled", Message: "link import was canceled"}
			continue
		}
		block, err := s.blockRepository.CreateBlockForAuthorizedPageAtOrderContext(workCtx, page.Id, outcome.block, order)
		if err != nil {
			outcome.block = nil
			outcome.issue = &ImportIssue{Index: candidate.index, URL: candidate.url, Code: "create_failed", Message: "link could not be saved"}
			continue
		}
		outcome.block = block
	}

	result := &ImportResult{Created: []mongomodels.BlockMongo{}, Warnings: []ImportIssue{}, Failed: []ImportIssue{}}
	for _, outcome := range outcomes {
		if outcome.block != nil {
			result.Created = append(result.Created, *outcome.block)
		} else if outcome.issue != nil && outcome.warn {
			result.Warnings = append(result.Warnings, *outcome.issue)
		} else if outcome.issue != nil {
			result.Failed = append(result.Failed, *outcome.issue)
		}
	}
	return result, nil
}

func validateImportURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || len(trimmed) > maxImportURLBytes {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Hostname() == "" || (!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
		return "", false
	}
	return trimmed, true
}

func normalizeURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Hostname() == "" {
		return trimmed
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}
	if parsed.Path == "/" {
		parsed.Path = ""
		parsed.RawPath = ""
	}
	parsed.Fragment = ""

	return parsed.String()
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
		updates["description"] = htmlsanitizer.SanitizeRichText(*body.Description)
	}

	if body.Tags != nil {
		updates["tags"] = *body.Tags
	}

	if body.Order != nil {
		if math.IsNaN(*body.Order) || math.IsInf(*body.Order, 0) {
			return nil, customerrors.ErrorInvalidBlock
		}
		updates["order"] = *body.Order
	}

	return s.blockRepository.UpdateBlock(blockId, ownerId, updates)
}

func (s *BlockService) DeleteBlock(blockId string, ownerId string) error {
	return s.blockRepository.DeleteBlock(blockId, ownerId)
}
