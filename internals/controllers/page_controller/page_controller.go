package pagecontroller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	pageservice "github.com/smil-thakur/save-the-link/internals/services/page_service"
)

type PageController struct {
	pageService *pageservice.PageService
}

func NewPageController(pageService *pageservice.PageService) *PageController {
	return &PageController{
		pageService: pageService,
	}
}

func ToPageResponseDTO(page mongomodels.PageMongo) dto.PageResponseDTO {
	var parentPageId *string

	if page.ParentPageId != nil {
		hex := page.ParentPageId.Hex()
		parentPageId = &hex
	}

	var deletedAt *string

	if page.DeletedAt != nil {
		formatted := page.DeletedAt.Format(time.RFC3339)
		deletedAt = &formatted
	}

	return dto.PageResponseDTO{
		Id:                 page.Id.Hex(),
		ParentPageId:       parentPageId,
		Title:              page.Title,
		Icon:               page.Icon,
		Layout:             page.Layout,
		Visibility:         page.Visibility,
		Collaboration:      page.Collaboration,
		Slug:               page.Slug,
		CollaboratorEmails: page.CollaboratorEmails,
		Order:              page.Order,
		CreatedAt:          page.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          page.UpdatedAt.Format(time.RFC3339),
		DeletedAt:          deletedAt,
	}
}

func (c *PageController) respondWithError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, customerrors.ErrorPageNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": err.Error()})
	case errors.Is(err, customerrors.ErrorForbidden):
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": err.Error()})
	case errors.Is(err, customerrors.ErrorInvalidPage):
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	case errors.Is(err, customerrors.ErrorCannotBookmarkOwnPage):
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("%v", err),
		})
	}
}

func (c *PageController) CreatePage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)

	var body dto.CreatePageDTO

	if err := ctx.ShouldBindBodyWithJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page, err := c.pageService.CreatePage(ownerId, body)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, ToPageResponseDTO(*page))
}

func (c *PageController) ListPages(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)

	pages, err := c.pageService.GetPages(ownerId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	response := make([]dto.PageResponseDTO, len(pages))

	for i, page := range pages {
		response[i] = ToPageResponseDTO(page)
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *PageController) GetPage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	page, err := c.pageService.GetPage(id, ownerId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, ToPageResponseDTO(*page))
}

func (c *PageController) UpdatePage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	var body dto.UpdatePageDTO

	if err := ctx.ShouldBindBodyWithJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page, err := c.pageService.UpdatePage(id, ownerId, body)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, ToPageResponseDTO(*page))
}

func (c *PageController) DeletePage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	if err := c.pageService.DeletePage(id, ownerId); err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *PageController) PublishPage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	var body dto.PublishPageDTO

	if err := ctx.ShouldBindBodyWithJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page, err := c.pageService.PublishPage(id, ownerId, body)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, ToPageResponseDTO(*page))
}

func (c *PageController) UnpublishPage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	page, err := c.pageService.UnpublishPage(id, ownerId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, ToPageResponseDTO(*page))
}

func (c *PageController) GetPublicPage(ctx *gin.Context) {
	slug := ctx.Param("slug")

	page, err := c.pageService.GetPublicPage(slug)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	// The collaborator list is only for the owner's Share dialog — anyone with
	// the link shouldn't be able to see who else was specifically invited.
	response := ToPageResponseDTO(*page)
	response.CollaboratorEmails = nil

	ctx.JSON(http.StatusOK, response)
}

func (c *PageController) ListTrash(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)

	pages, err := c.pageService.GetTrash(ownerId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	response := make([]dto.PageResponseDTO, len(pages))

	for i, page := range pages {
		response[i] = ToPageResponseDTO(page)
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *PageController) RestorePage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	page, err := c.pageService.RestorePage(id, ownerId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, ToPageResponseDTO(*page))
}

func (c *PageController) PermanentlyDeletePage(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	if err := c.pageService.PermanentlyDeletePage(id, ownerId); err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *PageController) SetCollaborators(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	var body dto.SetCollaboratorsDTO

	if err := ctx.ShouldBindBodyWithJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page, notFound, err := c.pageService.SetCollaborators(id, ownerId, body.Emails)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, dto.SetCollaboratorsResponseDTO{
		Page:           ToPageResponseDTO(*page),
		NotFoundEmails: notFound,
	})
}

func (c *PageController) ListPublicPages(ctx *gin.Context) {
	summaries, err := c.pageService.ListPublicPages(12)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	response := make([]dto.PublicPageSummaryDTO, len(summaries))

	for i, summary := range summaries {
		response[i] = dto.PublicPageSummaryDTO{
			Id:        summary.Id.Hex(),
			Title:     summary.Title,
			Icon:      summary.Icon,
			Slug:      summary.Slug,
			LinkCount: summary.LinkCount,
		}
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *PageController) BookmarkPage(ctx *gin.Context) {
	userId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	if err := c.pageService.Bookmark(id, userId); err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *PageController) UnbookmarkPage(ctx *gin.Context) {
	userId := ctx.MustGet("userId").(string)
	id := ctx.Param("id")

	if err := c.pageService.Unbookmark(id, userId); err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *PageController) ListBookmarks(ctx *gin.Context) {
	userId := ctx.MustGet("userId").(string)

	summaries, err := c.pageService.ListBookmarks(userId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	response := make([]dto.PublicPageSummaryDTO, len(summaries))

	for i, summary := range summaries {
		response[i] = dto.PublicPageSummaryDTO{
			Id:        summary.Id.Hex(),
			Title:     summary.Title,
			Icon:      summary.Icon,
			Slug:      summary.Slug,
			LinkCount: summary.LinkCount,
		}
	}

	ctx.JSON(http.StatusOK, response)
}
