package pageservice

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	userRepository  *repository.UserRepository
}

func NewPageService(pageRepository *repository.PageRepository, blockRepository *repository.BlockRepository, userRepository *repository.UserRepository) *PageService {
	return &PageService{
		pageRepository:  pageRepository,
		blockRepository: blockRepository,
		userRepository:  userRepository,
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
	if body.Collaboration != "view" && body.Collaboration != "edit" && body.Collaboration != "invite" {
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

func (s *PageService) ListPublicPages(limit int64) ([]repository.PublicPageSummary, error) {
	return s.pageRepository.ListPublicPages(limit)
}

// SetCollaborators resolves each email to a registered user (silently dropping
// ones that don't match — the client's autocomplete should only ever submit
// emails it found, but a free-typed unmatched email is reported back rather
// than erroring the whole request) and stores the result as the page's
// invite-only collaborator list.
func (s *PageService) SetCollaborators(id string, ownerId string, emails []string) (*mongomodels.PageMongo, []string, error) {
	ids := make([]bson.ObjectID, 0, len(emails))
	resolvedEmails := make([]string, 0, len(emails))
	var notFound []string

	seen := make(map[string]bool)

	for _, rawEmail := range emails {
		email := strings.TrimSpace(strings.ToLower(rawEmail))

		if email == "" || seen[email] {
			continue
		}
		seen[email] = true

		user, err := s.userRepository.FindUserByEmail(email)

		if err != nil {
			if errors.Is(err, customerrors.ErrorUserNotFound) {
				notFound = append(notFound, rawEmail)
				continue
			}
			return nil, nil, err
		}

		ids = append(ids, user.Id)
		resolvedEmails = append(resolvedEmails, user.Email)
	}

	updates := bson.M{
		"collaboratorIds":    ids,
		"collaboratorEmails": resolvedEmails,
	}

	page, err := s.pageRepository.UpdatePage(id, ownerId, updates)

	if err != nil {
		return nil, nil, err
	}

	return page, notFound, nil
}

// Bookmark lets an authenticated user save a public page they don't own for
// quick access from the sidebar. Rejected for a page's own owner — they
// already have it in their page tree, and bookmarking it would just be
// clutter and a duplicate entry point to the same page.
func (s *PageService) Bookmark(pageId string, userId string) error {
	page, err := s.pageRepository.GetPageByIdUnscoped(pageId)

	if err != nil {
		return err
	}

	if page.Visibility != "public" {
		return customerrors.ErrorForbidden
	}

	userObjectId, err := bson.ObjectIDFromHex(userId)

	if err != nil {
		return customerrors.ErrorUserNotFound
	}

	if page.OwnerId == userObjectId {
		return customerrors.ErrorCannotBookmarkOwnPage
	}

	return s.userRepository.AddBookmark(userId, pageId)
}

func (s *PageService) Unbookmark(pageId string, userId string) error {
	return s.userRepository.RemoveBookmark(userId, pageId)
}

func (s *PageService) ListBookmarks(userId string) ([]repository.PublicPageSummary, error) {
	user, err := s.userRepository.FindUserById(userId)

	if err != nil {
		return nil, err
	}

	return s.pageRepository.GetPageSummariesByIds(user.BookmarkedPageIds)
}

func generateSlug() (string, error) {
	buf := make([]byte, 6)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
