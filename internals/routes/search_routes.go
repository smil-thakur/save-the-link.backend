package routes

import (
	"github.com/gin-gonic/gin"
	searchcontroller "github.com/smil-thakur/save-the-link/internals/controllers/search_controller"
)

func SearchRoutes(r gin.IRoutes, searchController *searchcontroller.SearchController) {
	r.GET("/search", searchController.Search)
}
