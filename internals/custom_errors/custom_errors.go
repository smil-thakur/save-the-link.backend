package customerrors

import "errors"

var ErrorUserNotFound = errors.New("User does not exists")

var UserAlreadyExists = errors.New("An account with that email already exists")

var InvalidCredentials = errors.New("Invalid credentails provide")

var InvalidToken = errors.New("Invalid token")

var ErrorPageNotFound = errors.New("Page does not exist")

var ErrorForbidden = errors.New("You do not have access to this page")

var ErrorInvalidPage = errors.New("Invalid page data")

var ErrorBlockNotFound = errors.New("Link not found")

var ErrorInvalidBlock = errors.New("Invalid link data")

var ErrorInvalidURL = errors.New("That URL couldn't be reached")

var ErrorCannotBookmarkOwnPage = errors.New("You can't bookmark your own page")
