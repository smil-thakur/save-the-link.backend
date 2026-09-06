package blockcontroller

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	blockservice "github.com/smil-thakur/save-the-link/internals/services/block_service"
	pageservice "github.com/smil-thakur/save-the-link/internals/services/page_service"
)

type BlockController struct {
	blockService *blockservice.BlockService
	pageService  *pageservice.PageService
}

func NewBlockController(blockService *blockservice.BlockService, pageService *pageservice.PageService) *BlockController {
	return &BlockController{
		blockService: blockService,
		pageService:  pageService,
	}
}

func ToBlockResponseDTO(block mongomodels.BlockMongo) dto.BlockResponseDTO {
	tags := block.Tags

	if tags == nil {
		tags = []string{}
	}

	return dto.BlockResponseDTO{
		Id:          block.Id.Hex(),
		PageId:      block.PageId.Hex(),
		URL:         block.URL,
		Title:       block.Title,
		Description: block.Description,
		CoverImage:  block.CoverImage,
		Favicon:     block.Favicon,
		SiteName:    block.SiteName,
		FetchStatus: block.FetchStatus,
		Tags:        tags,
		Order:       block.Order,
		CreatedAt:   block.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   block.UpdatedAt.Format(time.RFC3339),
	}
}

func (c *BlockController) respondWithError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, customerrors.ErrorPageNotFound), errors.Is(err, customerrors.ErrorBlockNotFound):
		ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": err.Error()})
	case errors.Is(err, customerrors.ErrorForbidden):
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": err.Error()})
	case errors.Is(err, customerrors.ErrorInvalidBlock), errors.Is(err, customerrors.ErrorInvalidPage):
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("%v", err),
		})
	}
}

func (c *BlockController) CreateBlock(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	pageId := ctx.Param("id")

	var body dto.CreateBlockDTO

	if err := ctx.ShouldBindBodyWithJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	block, err := c.blockService.CreateBlock(ctx.Request.Context(), pageId, ownerId, body.URL)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, ToBlockResponseDTO(*block))
}

func (c *BlockController) ListBlocks(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	pageId := ctx.Param("id")

	blocks, err := c.blockService.GetBlocks(pageId, ownerId)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	response := make([]dto.BlockResponseDTO, len(blocks))

	for i, block := range blocks {
		response[i] = ToBlockResponseDTO(block)
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *BlockController) UpdateBlock(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	blockId := ctx.Param("blockId")

	var body dto.UpdateBlockDTO

	if err := ctx.ShouldBindBodyWithJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	block, err := c.blockService.UpdateBlock(blockId, ownerId, body)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, ToBlockResponseDTO(*block))
}

func (c *BlockController) DeleteBlock(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	blockId := ctx.Param("blockId")

	if err := c.blockService.DeleteBlock(blockId, ownerId); err != nil {
		c.respondWithError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (c *BlockController) ListPublicBlocks(ctx *gin.Context) {
	slug := ctx.Param("slug")

	page, err := c.pageService.GetPublicPage(slug)

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	blocks, err := c.blockService.GetPublicBlocks(page.Id.Hex())

	if err != nil {
		c.respondWithError(ctx, err)
		return
	}

	response := make([]dto.BlockResponseDTO, len(blocks))

	for i, block := range blocks {
		response[i] = ToBlockResponseDTO(block)
	}

	ctx.JSON(http.StatusOK, response)
}
