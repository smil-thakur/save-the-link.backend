package searchcontroller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	blockcontroller "github.com/smil-thakur/save-the-link/internals/controllers/block_controller"
	pagecontroller "github.com/smil-thakur/save-the-link/internals/controllers/page_controller"
	searchservice "github.com/smil-thakur/save-the-link/internals/services/search_service"
)

type SearchController struct {
	searchService *searchservice.SearchService
}

func NewSearchController(searchService *searchservice.SearchService) *SearchController {
	return &SearchController{
		searchService: searchService,
	}
}

func (c *SearchController) Search(ctx *gin.Context) {
	ownerId := ctx.MustGet("userId").(string)
	query := ctx.Query("q")

	pages, blocks, err := c.searchService.Search(ownerId, query)

	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	pageResults := make([]dto.PageResponseDTO, len(pages))

	for i, page := range pages {
		pageResults[i] = pagecontroller.ToPageResponseDTO(page)
	}

	blockResults := make([]dto.BlockResponseDTO, len(blocks))

	for i, block := range blocks {
		blockResults[i] = blockcontroller.ToBlockResponseDTO(block)
	}

	ctx.JSON(http.StatusOK, dto.SearchResultsDTO{
		Pages:  pageResults,
		Blocks: blockResults,
	})
}
