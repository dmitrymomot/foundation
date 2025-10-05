package sessiontransport

import "errors"

var (
	// ErrNoToken is returned when no authentication token is present in the request
	ErrNoToken = errors.New("sessiontransport: no token")

	// ErrInvalidToken is returned when the token format or signature is invalid
	ErrInvalidToken = errors.New("sessiontransport: invalid token")
)
