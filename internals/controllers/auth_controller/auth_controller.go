package authcontroller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	dto "github.com/smil-thakur/save-the-link/internals/DTO"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
	jwtservice "github.com/smil-thakur/save-the-link/internals/jwt_service"
	"github.com/smil-thakur/save-the-link/internals/models"
	authservice "github.com/smil-thakur/save-the-link/internals/services/auth_service"
	pageservice "github.com/smil-thakur/save-the-link/internals/services/page_service"
)

type AuthController struct {
	authService *authservice.AuthService
	pageService *pageservice.PageService
	jwtService  *jwtservice.JWTService
}

func NewAuthController(authService *authservice.AuthService, pageService *pageservice.PageService, jwtService *jwtservice.JWTService) *AuthController {
	return &AuthController{
		authService: authService,
		pageService: pageService,
		jwtService:  jwtService,
	}
}

func (a *AuthController) RegisterUser(ctx *gin.Context, username string, password string, email string) *models.User {

	id, err := a.authService.RegisterNewUser(username, email, password)

	if err != nil {
		if errors.Is(err, customerrors.UserAlreadyExists) {
			ctx.IndentedJSON(http.StatusConflict, gin.H{
				"message": fmt.Sprintf("%v", err),
			})
			return nil
		}
		ctx.IndentedJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("%v", err),
		})
		return nil
	}

	user := models.User{
		ID:       id,
		Username: username,
		Email:    email,
	}

	ctx.IndentedJSON(http.StatusCreated, user)

	return &user
}

func (a *AuthController) LogoutUser(ctx *gin.Context) {
	// SameSite=None (requires Secure) so the browser still sends/clears these
	// cookies when the frontend is hosted on a different domain than the API.
	ctx.SetSameSite(http.SameSiteNoneMode)
	ctx.SetCookie("access_token", "", -1, "/", "", true, true)
	ctx.SetCookie("refresh_token", "", -1, "/", "", true, true)

	ctx.Status(http.StatusNoContent)
}

func (a *AuthController) Me(ctx *gin.Context) {
	userId, ok := ctx.Get("userId")

	if !ok {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "authentication required",
		})
		return
	}

	user, err := a.authService.GetUserById(userId.(string))

	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "authentication required",
		})
		return
	}

	ctx.IndentedJSON(http.StatusOK, dto.LoginUserResponseDTO{
		Email:    user.Email,
		Username: user.Username,
	})
}

func (a *AuthController) DeleteAccount(ctx *gin.Context) {
	userId := ctx.MustGet("userId").(string)

	if err := a.pageService.DeleteAllPagesForOwner(userId); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("%v", err),
		})
		return
	}

	if err := a.authService.DeleteAccount(userId); err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("%v", err),
		})
		return
	}

	ctx.SetSameSite(http.SameSiteNoneMode)
	ctx.SetCookie("access_token", "", -1, "/", "", true, true)
	ctx.SetCookie("refresh_token", "", -1, "/", "", true, true)

	ctx.Status(http.StatusNoContent)
}

func (a *AuthController) LoginUser(ctx *gin.Context, email string, password string) *dto.LoginUserResponseDTO {

	user, err := a.authService.LoginUser(email, password)

	if err != nil {
		if errors.Is(err, customerrors.ErrorUserNotFound) || errors.Is(err, customerrors.InvalidCredentials) {
			// Deliberately the same message for both cases so a failed login can't be
			// used to enumerate which emails have an account.
			ctx.IndentedJSON(http.StatusUnauthorized, gin.H{
				"message": "Invalid email or password",
			})
			return nil
		}
		ctx.IndentedJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("Internal server error %v", err),
		})
		return nil
	}

	token, err := a.jwtService.CreateToken(user.Id.Hex())
	refresh_token, err := a.jwtService.CreateRefreshToken(user.Id.Hex())

	if err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, gin.H{
			"message": fmt.Sprintf("%v", err),
		})
		return nil
	}

	ctx.SetSameSite(http.SameSiteNoneMode)
	ctx.SetCookie(
		"access_token",
		token,
		900,
		"/",
		"",
		true,
		true,
	)

	ctx.SetCookie(
		"refresh_token",
		refresh_token,
		604800,
		"/",
		"",
		true,
		true,
	)

	resp := &dto.LoginUserResponseDTO{
		Email:    user.Email,
		Username: user.Username,
	}

	ctx.IndentedJSON(http.StatusOK, resp)

	return resp

}
