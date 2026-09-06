package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

type PasswordManager struct {
}

func NewPasswordManager() *PasswordManager {
	return &PasswordManager{}
}

func (p *PasswordManager) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func (p *PasswordManager) ComparePassword(hashedPassword string, password string) bool {

	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))

	if err != nil {
		return false
	}

	return true
}
