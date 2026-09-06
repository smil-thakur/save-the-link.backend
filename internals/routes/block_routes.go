package routes

import (
	"github.com/gin-gonic/gin"
	blockcontroller "github.com/smil-thakur/save-the-link/internals/controllers/block_controller"
)

func BlockRoutes(r gin.IRoutes, blockController *blockcontroller.BlockController) {
	r.POST("/pages/:id/blocks", blockController.CreateBlock)
	r.GET("/pages/:id/blocks", blockController.ListBlocks)
	r.PATCH("/blocks/:blockId", blockController.UpdateBlock)
	r.DELETE("/blocks/:blockId", blockController.DeleteBlock)
}

func PublicBlockRoutes(r gin.IRoutes, blockController *blockcontroller.BlockController) {
	r.GET("/public/pages/:slug/blocks", blockController.ListPublicBlocks)
}
