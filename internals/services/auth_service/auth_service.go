package authservice

import (
	mongomodels "github.com/smil-thakur/save-the-link/internals/mongo_models"
	"github.com/smil-thakur/save-the-link/internals/repository"
)

type AuthService struct {
	userRepository *repository.UserRepository
}

func NewAuthService(userRepository *repository.UserRepository) *AuthService {
	return &AuthService{
		userRepository: userRepository,
	}
}

func (s *AuthService) RegisterNewUser(username string, email string, password string) (string, error) {
	id, err := s.userRepository.RegisterUserToDatabase(username, email, password)

	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *AuthService) GetUserById(id string) (*mongomodels.UserMongo, error) {
	user, err := s.userRepository.FindUserById(id)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) LoginUser(email string, password string) (*mongomodels.UserMongo, error) {

	user, err := s.userRepository.LoginUser(email, password)

	if err != nil {
		return nil, err
	}

	return user, nil

}

func (s *AuthService) DeleteAccount(userId string) error {
	return s.userRepository.DeleteUser(userId)
}
