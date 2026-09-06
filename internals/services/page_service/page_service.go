package pageservice

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"

	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type PageService struct {
	pageRepository  *repository.PageRepository
	blockRepository *repository.BlockRepository
}

func NewPageService(pageRepository *repository.PageRepository, blockRepository *repository.BlockRepository) *PageService {
	return &PageService{
		pageRepository:  pageRepository,
		blockRepository: blockRepository,
	}
}

func (s *PageService) CreatePage(ownerId string, body dto.CreatePageDTO) (*mongomodels.PageMongo, error) {
	title := strings.TrimSpace(body.Title)

	if title == "" {
		return nil, customerrors.ErrorInvalidPage
	}

	return s.pageRepository.CreatePage(ownerId, title, body.ParentPageId)
}

func (s *PageService) GetPages(ownerId string) ([]mongomodels.PageMongo, error) {
	return s.pageRepository.GetPagesByOwner(ownerId)
}

func (s *PageService) GetPage(id string, ownerId string) (*mongomodels.PageMongo, error) {
	return s.pageRepository.GetPageById(id, ownerId)
}

func (s *PageService) UpdatePage(id string, ownerId string, body dto.UpdatePageDTO) (*mongomodels.PageMongo, error) {
	updates := bson.M{}

	if body.Title != nil {
		title := strings.TrimSpace(*body.Title)

		if title == "" {
			return nil, customerrors.ErrorInvalidPage
		}

		updates["title"] = title
	}

	if body.Icon != nil {
		updates["icon"] = *body.Icon
	}

	if body.Order != nil {
		updates["order"] = *body.Order
	}

	if body.ParentPageId != nil {
		if *body.ParentPageId == "" {
			updates["parentPageId"] = nil
		} else {
			if *body.ParentPageId == id {
				return nil, customerrors.ErrorInvalidPage
			}

			descendantIds, err := s.pageRepository.GetDescendantIds(id)

			if err != nil {
				return nil, err
			}

			if slices.Contains(descendantIds, *body.ParentPageId) {
				return nil, customerrors.ErrorInvalidPage
			}

			parentObjectId, err := bson.ObjectIDFromHex(*body.ParentPageId)

			if err != nil {
				return nil, customerrors.ErrorInvalidPage
			}

			// Ensures the new parent exists and is owned by the caller.
			if _, err := s.pageRepository.GetPageById(*body.ParentPageId, ownerId); err != nil {
				return nil, err
			}

			updates["parentPageId"] = parentObjectId
		}
	}

	return s.pageRepository.UpdatePage(id, ownerId, updates)
}

func (s *PageService) DeletePage(id string, ownerId string) error {
	return s.pageRepository.DeletePage(id, ownerId)
}

func (s *PageService) GetTrash(ownerId string) ([]mongomodels.PageMongo, error) {
	return s.pageRepository.GetTrashedPages(ownerId)
}

func (s *PageService) RestorePage(id string, ownerId string) (*mongomodels.PageMongo, error) {
	return s.pageRepository.RestorePage(id, ownerId)
}

func (s *PageService) PermanentlyDeletePage(id string, ownerId string) error {
	return s.pageRepository.PermanentlyDeletePage(id, ownerId, s.blockRepository)
}

func (s *PageService) DeleteAllPagesForOwner(ownerId string) error {
	return s.pageRepository.DeleteAllForOwner(ownerId, s.blockRepository)
}

func (s *PageService) PublishPage(id string, ownerId string, body dto.PublishPageDTO) (*mongomodels.PageMongo, error) {
	if body.Collaboration != "view" && body.Collaboration != "edit" {
		return nil, customerrors.ErrorInvalidPage
	}

	page, err := s.pageRepository.GetPageById(id, ownerId)

	if err != nil {
		return nil, err
	}

	updates := bson.M{
		"visibility":    "public",
		"collaboration": body.Collaboration,
	}

	if page.Slug == nil {
		slug, err := generateSlug()

		if err != nil {
			return nil, err
		}

		updates["slug"] = slug
	}

	return s.pageRepository.UpdatePage(id, ownerId, updates)
}

func (s *PageService) UnpublishPage(id string, ownerId string) (*mongomodels.PageMongo, error) {
	updates := bson.M{
		"visibility":    "private",
		"collaboration": "none",
		"slug":          nil,
	}

	return s.pageRepository.UpdatePage(id, ownerId, updates)
}

func (s *PageService) GetPublicPage(slug string) (*mongomodels.PageMongo, error) {
	return s.pageRepository.GetPublicPageBySlug(slug)
}

func generateSlug() (string, error) {
	buf := make([]byte, 6)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
