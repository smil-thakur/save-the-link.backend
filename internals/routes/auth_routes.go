package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	authcontroller "github.com/smil-thakur/save-the-link/internals/controllers/auth_controller"
)

func AuthRoutes(r *gin.Engine, authController *authcontroller.AuthController) {
	r.POST("/register", func(ctx *gin.Context) {

		var userDto dto.UserDTO

		if err := ctx.ShouldBindBodyWithJSON(&userDto); err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{
					"error": err.Error(),
				},
			)
			return
		}

		authController.RegisterUser(ctx, userDto.Username, userDto.Password, userDto.Email)
	})
	r.POST("/login", func(ctx *gin.Context) {
		var loginUserDto dto.LoginUserDTO

		if err := ctx.ShouldBindBodyWithJSON(&loginUserDto); err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{
					"error": err.Error(),
				},
			)
			return
		}

		authController.LoginUser(ctx, loginUserDto.Email, loginUserDto.Password)

	})
	r.POST("/logout", authController.LogoutUser)
}
