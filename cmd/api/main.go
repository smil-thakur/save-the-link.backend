package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/smil-thakur/save-the-link/internals/config"
	authcontroller "github.com/smil-thakur/save-the-link/internals/controllers/auth_controller"
	blockcontroller "github.com/smil-thakur/save-the-link/internals/controllers/block_controller"
	pagecontroller "github.com/smil-thakur/save-the-link/internals/controllers/page_controller"
	searchcontroller "github.com/smil-thakur/save-the-link/internals/controllers/search_controller"
	"github.com/smil-thakur/save-the-link/internals/crypto"
	jwtservice "github.com/smil-thakur/save-the-link/internals/jwt_service"
	"github.com/smil-thakur/save-the-link/internals/middleware"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"github.com/smil-thakur/save-the-link/internals/routes"
	authservice "github.com/smil-thakur/save-the-link/internals/services/auth_service"
	blockservice "github.com/smil-thakur/save-the-link/internals/services/block_service"
	pageservice "github.com/smil-thakur/save-the-link/internals/services/page_service"
	searchservice "github.com/smil-thakur/save-the-link/internals/services/search_service"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	r := gin.Default()
	config := config.GetConfig()

	client, err := mongo.Connect(options.Client().ApplyURI(config.Mongo_db_connection_string))
	passwordManager := crypto.NewPasswordManager()
	jwtService := jwtservice.NewJWTService(config.JWT_secret)

	userRepository := repository.NewUserRepository(client, context.Background(), passwordManager)
	authService := authservice.NewAuthService(userRepository)

	pageRepository := repository.NewPageRepository(client, context.Background())
	blockRepository := repository.NewBlockRepository(client, context.Background(), pageRepository)

	pageService := pageservice.NewPageService(pageRepository, blockRepository)
	pageController := pagecontroller.NewPageController(pageService)

	blockService := blockservice.NewBlockService(blockRepository)
	blockController := blockcontroller.NewBlockController(blockService, pageService)

	searchService := searchservice.NewSearchService(pageRepository, blockRepository)
	searchController := searchcontroller.NewSearchController(searchService)

	authController := authcontroller.NewAuthController(authService, pageService, jwtService)

	if err != nil {
		log.Fatalf("Unable to connect to mongo db %v", err)
	}

	// Registered before any routes: Gin bakes the middleware chain in at route
	// registration time, so CORS added after a route (e.g. after /ping) would
	// never actually apply to it.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     config.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	routes.AuthRoutes(r, authController)

	r.GET("/me", middleware.AuthMiddleWare(jwtService), authController.Me)
	r.DELETE("/me", middleware.AuthMiddleWare(jwtService), authController.DeleteAccount)

	routes.PublicPageRoutes(r, pageController)
	routes.PublicBlockRoutes(r, blockController)

	protected := r.Group("/", middleware.AuthMiddleWare(jwtService))
	routes.PageRoutes(protected, pageController)
	routes.BlockRoutes(protected, blockController)
	routes.SearchRoutes(protected, searchController)

	r.Run()
}
