package customerrors

import "errors"

var ErrorUserNotFound = errors.New("User does not exists")

var UserAlreadyExists = errors.New("User Already Exists")

var InvalidCredentials = errors.New("Invalid credentails provide")

var InvalidToken = errors.New("Invalid token")
