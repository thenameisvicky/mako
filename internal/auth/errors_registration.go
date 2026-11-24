package auth

import (
	"errors"
	"net/http"
)

var (
	errInvalidCredentials = errors.New("invalid credentials")
	errTeamRequired       = errors.New("teamId is required to create a new user")
	errTeamNameRequired   = errors.New("a team name is required to create a new team")
	errTeamNotFound       = errors.New("specified teamId does not exist")
)

// registrationError maps a registration failure to a client-safe response.
// Unknown errors are deliberately collapsed so internals are not leaked.
func registrationError(err error) (int, string) {
	switch {
	case errors.Is(err, errInvalidCredentials):
		return http.StatusUnauthorized, "Invalid credentials"
	case errors.Is(err, errTeamRequired):
		return http.StatusBadRequest, "Enter a team ID, or tick \"create a new team\""
	case errors.Is(err, errTeamNameRequired):
		return http.StatusBadRequest, "Enter a name for your new team"
	case errors.Is(err, errTeamNotFound):
		return http.StatusBadRequest, "Specified teamId does not exist"
	default:
		return http.StatusInternalServerError, "Failed to create user"
	}
}
