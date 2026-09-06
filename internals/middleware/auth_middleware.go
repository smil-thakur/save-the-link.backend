package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	jwtservice "github.com/smil-thakur/save-the-link/internals/jwt_service"
)

func AuthMiddleWare(jwtservice *jwtservice.JWTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken, err := ctx.Cookie("access_token")

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authentication required",
			})
			return
		}

		token, err := jwtservice.ValidateAccessToken(accessToken)

		if err == nil {
			claims, ok := token.Claims.(jwt.MapClaims)

			if !ok {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"message": "invalid access token",
				})
				return
			}

			userId, ok := claims["sub"].(string)

			if !ok {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"message": "invalid access token",
				})
				return
			}

			ctx.Set("userId", userId)
			ctx.Next()
			return
		}

		if !errors.Is(err, jwt.ErrTokenExpired) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid access token",
			})
			return
		}

		refreshToken, err := ctx.Cookie("refresh_token")

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "session expired",
			})
			return
		}

		refreshJWT, err := jwtservice.ValidateRefreshToken(refreshToken)

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid refresh token",
			})
			return
		}

		claims, ok := refreshJWT.Claims.(jwt.MapClaims)

		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid refresh token",
			})
			return
		}

		userId, ok := claims["sub"].(string)

		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid refresh token",
			})
			return
		}

		newAccessToken, err := jwtservice.CreateToken(userId)

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("Unable to create new Access token %v", err),
			})
			return
		}

		newRefreshToken, err := jwtservice.CreateRefreshToken(userId)

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("Unable to create new Refresh token %v", err),
			})
		}

		ctx.SetCookie(
			"access_token",
			newAccessToken,
			900,
			"/",
			"",
			true,
			true,
		)

		ctx.SetCookie(
			"refresh_token",
			newRefreshToken,
			604800,
			"/",
			"",
			true,
			true,
		)
		ctx.Set("userId", userId)
		ctx.Next()

	}
}
