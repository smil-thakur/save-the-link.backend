package dto

type UserDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginUserDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginUserResponseDTO struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}
