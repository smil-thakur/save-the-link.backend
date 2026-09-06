package routes

import (
	"github.com/gin-gonic/gin"
	pagecontroller "github.com/smil-thakur/save-the-link/internals/controllers/page_controller"
)

func PageRoutes(r gin.IRoutes, pageController *pagecontroller.PageController) {
	r.POST("/pages", pageController.CreatePage)
	r.GET("/pages", pageController.ListPages)
	r.GET("/pages/:id", pageController.GetPage)
	r.PATCH("/pages/:id", pageController.UpdatePage)
	r.DELETE("/pages/:id", pageController.DeletePage)
	r.POST("/pages/:id/publish", pageController.PublishPage)
	r.POST("/pages/:id/unpublish", pageController.UnpublishPage)
	r.GET("/trash", pageController.ListTrash)
	r.POST("/pages/:id/restore", pageController.RestorePage)
	r.DELETE("/pages/:id/permanent", pageController.PermanentlyDeletePage)
}

func PublicPageRoutes(r gin.IRoutes, pageController *pagecontroller.PageController) {
	r.GET("/public/pages/:slug", pageController.GetPublicPage)
}
