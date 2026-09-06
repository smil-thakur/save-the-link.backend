package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/smil-thakur/save-the-link/internals/config"
	authcontroller "github.com/smil-thakur/save-the-link/internals/controllers/auth_controller"
	"github.com/smil-thakur/save-the-link/internals/crypto"
	jwtservice "github.com/smil-thakur/save-the-link/internals/jwt_service"
	"github.com/smil-thakur/save-the-link/internals/middleware"
	"github.com/smil-thakur/save-the-link/internals/repository"
	"github.com/smil-thakur/save-the-link/internals/routes"
	authservice "github.com/smil-thakur/save-the-link/internals/services/auth_service"
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
	authController := authcontroller.NewAuthController(authService, jwtService)

	if err != nil {
		log.Fatalf("Unable to connect to mongo db %v", err)
	}

	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	routes.AuthRoutes(r, authController)

	r.GET("/me", middleware.AuthMiddleWare(jwtService), authController.Me)

	r.Run()
}
