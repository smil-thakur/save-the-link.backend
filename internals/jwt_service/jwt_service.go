package jwtservice

import (
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/smil-thakur/save-the-link/internals/constants"
	customerrors "github.com/smil-thakur/save-the-link/internals/custom_errors"
)

type JWTService struct {
	Jwt_secret string
}

func NewJWTService(jwt_secret string) *JWTService {
	return &JWTService{
		Jwt_secret: jwt_secret,
	}
}

func (s *JWTService) CreateToken(userId string) (string, error) {
	tokenMaker := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userId,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
		"type": constants.AccessToken,
	})

	token, err := tokenMaker.SignedString([]byte(s.Jwt_secret))

	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *JWTService) ValidateAccessToken(token string) (*jwt.Token, error) {
	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, customerrors.InvalidToken
		}
		return []byte(s.Jwt_secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return parsedToken, jwt.ErrTokenExpired
		}
		return nil, err
	}

	if !parsedToken.Valid {
		return nil, customerrors.InvalidToken
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)

	if !ok {
		return nil, customerrors.InvalidToken
	}

	if claims["type"] != string(constants.AccessToken) {
		return nil, customerrors.InvalidToken
	}

	return parsedToken, nil
}

func (s *JWTService) CreateRefreshToken(userId string) (string, error) {
	tokenMaker := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userId,
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(24 * 7 * time.Hour).Unix(),
		"type": constants.RefreshToken,
	})

	token, err := tokenMaker.SignedString([]byte(s.Jwt_secret))

	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *JWTService) ValidateRefreshToken(token string) (*jwt.Token, error) {
	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, customerrors.InvalidToken
		}
		return []byte(s.Jwt_secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return parsedToken, jwt.ErrTokenExpired
		}
		return nil, err
	}

	if !parsedToken.Valid {
		return nil, customerrors.InvalidToken
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)

	if !ok {
		return nil, customerrors.InvalidToken
	}

	if claims["type"] != string(constants.RefreshToken) {
		return nil, customerrors.InvalidToken
	}

	return parsedToken, nil
}
